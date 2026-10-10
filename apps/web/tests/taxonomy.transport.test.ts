import assert from 'node:assert/strict';
import test from 'node:test';

import type { requestAuthAPI } from '../src/api/auth/auth.server.ts';
import { forwardTaxonomyManagement } from '../src/api/taxonomy/taxonomy.proxy.server.ts';
import { parsePreview, parseTaxonomy } from '../src/api/taxonomy/taxonomy.types.ts';
import { validManagementPayload } from '../src/api/taxonomy/taxonomy.validation.ts';

const tagID = '019f033c-2111-7000-9000-000000000001';
const revision = 'a'.repeat(64);
const otherID = '019f033c-2111-7000-9000-000000000002';
const tagInput = { name: '中文标签', description: '', expected_revision: revision };
const headers = {
  Cookie: 'heyblog_access_token=test-session',
  Origin: 'https://web.example.test',
  'Sec-Fetch-Site': 'same-origin',
  'Content-Type': 'application/json',
};

function mutation(
  body: unknown = tagInput,
  overrides: Readonly<Record<string, string>> = {},
): Request {
  return new Request('https://web.example.test/management/tags/data', {
    method: 'POST',
    headers: { ...headers, ...overrides },
    body: JSON.stringify(body),
  });
}

function backend(role = 'ADMIN', permissions: readonly string[] = ['taxonomy.manage']) {
  const calls: { path: string; body: unknown }[] = [];
  const upstream: typeof requestAuthAPI = async (_request, path, init) => {
    calls.push({ path, body: init?.body });
    return path === '/auth/me'
      ? Response.json(
          { user: { role, permissions } },
          {
            headers: { 'Set-Cookie': 'heyblog_access_token=renewed; HttpOnly; Path=/' },
          },
        )
      : Response.json(
          { code: 'rate_limited' },
          {
            status: 429,
            headers: { 'Retry-After': '60', Location: 'https://unsafe.example.test' },
          },
        );
  };
  return { calls, upstream };
}

test('tag creation rejects cross-origin requests before authentication', async () => {
  const cases: Readonly<Record<string, string>>[] = [
    { Origin: 'https://attacker.example.test' },
    { 'Sec-Fetch-Site': 'cross-site' },
    { 'Sec-Fetch-Site': 'same-site' },
    { Origin: '' },
    { 'Sec-Fetch-Site': '' },
  ];
  for (const overrides of cases) {
    const stub = backend();
    const response = await forwardTaxonomyManagement(
      mutation(tagInput, overrides),
      '',
      stub.upstream,
    );
    assert.equal(response.status, 403);
    assert.equal(stub.calls.length, 0);
  }
});

test('tag creation accepts only bounded business inputs', async () => {
  for (const body of [
    { ...tagInput, model_id: 'paid-model' },
    { ...tagInput, prompt: 'ignore all rules' },
    { ...tagInput, endpoint: 'https://unsafe.example.test' },
    { ...tagInput, name: '文'.repeat(129) },
    { ...tagInput, description: 'x'.repeat(2001) },
    { ...tagInput, tag_id: 'invalid' },
    { ...tagInput, name: ' ' },
  ]) {
    const stub = backend();
    assert.equal((await forwardTaxonomyManagement(mutation(body), '', stub.upstream)).status, 422);
    assert.equal(stub.calls.length, 0);
  }
  const stub = backend();
  const oversized = new Request('https://web.example.test/management/tags/data', {
    method: 'POST',
    headers,
    body: ' '.repeat(65537) + JSON.stringify(tagInput),
  });
  assert.equal((await forwardTaxonomyManagement(oversized, '', stub.upstream)).status, 422);
  assert.equal(stub.calls.length, 0);
});

test('authorization requires taxonomy permission', async () => {
  for (const [role, permissions] of [
    ['USER', ['taxonomy.manage']],
    ['ADMIN', []],
  ] as const) {
    const stub = backend(role, permissions);
    assert.equal((await forwardTaxonomyManagement(mutation(), '', stub.upstream)).status, 403);
    assert.equal(stub.calls.length, 1);
  }
  const stub = backend();
  assert.equal(
    (
      await forwardTaxonomyManagement(
        new Request('https://web.example.test/management/tags/data'),
        '',
        stub.upstream,
      )
    ).status,
    401,
  );
});

test('authorized forwarding preserves quotas, refresh cookies and no-store, removes redirects', async () => {
  const stub = backend();
  const response = await forwardTaxonomyManagement(mutation(), '', stub.upstream);
  assert.deepEqual(stub.calls, [
    { path: '/auth/me', body: undefined },
    {
      path: '/management/taxonomy/tags',
      body: tagInput,
    },
  ]);
  assert.equal(response.status, 429);
  assert.equal(response.headers.get('Retry-After'), '60');
  assert.equal(response.headers.get('Location'), null);
  assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
  assert.equal(response.headers.getSetCookie().length, 1);
});

test('proxy rejects arbitrary paths, methods, query parameters and malformed auth responses', async () => {
  const stub = backend('SYS_ADMIN');
  assert.equal(
    (await forwardTaxonomyManagement(mutation(), 'other/endpoint', stub.upstream)).status,
    404,
  );
  assert.equal(
    (
      await forwardTaxonomyManagement(
        new Request('https://web.example.test/management/tags/data', { method: 'PATCH' }),
        '',
        stub.upstream,
      )
    ).status,
    405,
  );
  assert.equal(
    (
      await forwardTaxonomyManagement(
        new Request('https://web.example.test/management/tags/data?endpoint=other'),
        '',
        stub.upstream,
      )
    ).status,
    400,
  );
  assert.equal(stub.calls.length, 0);
  assert.equal(
    (await forwardTaxonomyManagement(mutation(), '', async () => new Response('{broken'))).status,
    502,
  );
});

test('tag and structural payload validators preserve optional descriptions and revision protection', () => {
  const create = {
    name: 'New tag',
    description: '',
    expected_revision: revision,
  };
  assert.equal(validManagementPayload(create, '', 'POST'), true);
  assert.equal(validManagementPayload({ ...create, slug: '中文' }, '', 'POST'), false);
  assert.equal(
    validManagementPayload({ ...create, expected_revision: '9223372036854775808' }, '', 'POST'),
    false,
  );
  const change = {
    kind: 'merge',
    source_id: tagID,
    target_id: otherID,
    expected_revision: revision,
  };
  assert.equal(validManagementPayload(change, 'changes/preview', 'POST'), true);
  assert.equal(validManagementPayload(change, 'changes/apply', 'POST'), false);
  assert.equal(
    validManagementPayload(
      { ...change, fingerprint: 'preview-fingerprint' },
      'changes/apply',
      'POST',
    ),
    true,
  );
  assert.equal(
    validManagementPayload(
      { ...change, replacements: [{ cascade_id: tagID, target_cascade_id: 'bad' }] },
      'changes/preview',
      'POST',
    ),
    false,
  );
});

test('DTO parsers accept name-only tags and reject malformed successes', () => {
  const tag = {
    id: tagID,
    name: '中文标签',
    description: '',
    roles: ['TERTIARY'],
    is_enabled: false,
    site_count: 2,
    article_count: 0,
  };
  assert.ok(parseTaxonomy({ tags: [tag], cascades: [], revision }));
  assert.equal(parseTaxonomy({ tags: [{ ...tag, site_count: -1 }], cascades: [], revision }), null);
  assert.equal(parseTaxonomy({ tags: [tag], cascades: [], revision: 2 }), null);
  const preview = {
    revision,
    fingerprint: 'hash',
    site_count: 1,
    article_count: 0,
    removed_duplicates: 0,
    blockers: [],
    paths: [{ cascade_id: tagID, scope: 'SITE', label: '中文 / JavaScript' }],
  };
  assert.deepEqual(parsePreview(preview), preview);
  assert.equal(
    parsePreview({
      revision,
      fingerprint: 'hash',
      site_count: 1,
      article_count: 0,
      removed_duplicates: 0,
      blockers: [],
      paths: [{ cascade_id: 'bad', scope: 'SITE', label: '路径' }],
    }),
    null,
  );
});

test('management accepts API field limits without truncation', () => {
  const fields = {
    name: '文'.repeat(120),
    description: '文'.repeat(2000),
    is_enabled: true,
    expected_revision: revision,
  };
  assert.equal(validManagementPayload(fields, tagID, 'PUT'), true);
  assert.equal(validManagementPayload({ ...fields, name: '文'.repeat(121) }, tagID, 'PUT'), false);
  assert.equal(
    validManagementPayload({ ...fields, description: '文'.repeat(2001) }, tagID, 'PUT'),
    false,
  );
});

test('dictionary and path requests accept shared endpoints and reject retired hierarchy operations', () => {
  assert.equal(
    validManagementPayload(
      {
        scope: 'SITE',
        primary_id: tagID,
        secondary_id: tagID,
        taxonomy_key: 'other/topic-other-other',
        expected_revision: revision,
      },
      'cascades',
      'POST',
    ),
    true,
  );
  assert.equal(
    validManagementPayload(
      {
        kind: 'path_update',
        source_id: tagID,
        primary_id: otherID,
        secondary_id: otherID,
        is_enabled: false,
        expected_revision: revision,
      },
      'changes/preview',
      'POST',
    ),
    true,
  );
  assert.equal(
    validManagementPayload(
      { kind: 'path_merge', source_id: tagID, target_id: otherID, expected_revision: revision },
      'changes/preview',
      'POST',
    ),
    true,
  );
  assert.equal(
    validManagementPayload(
      { kind: 'reparent', source_id: tagID, parent_id: otherID, expected_revision: revision },
      'changes/preview',
      'POST',
    ),
    false,
  );
  assert.equal(
    validManagementPayload(
      {
        name: '其他',
        slug: 'other',
        description: '',
        taxonomy_level: 1,
        expected_revision: revision,
      },
      '',
      'POST',
    ),
    false,
  );
});

test('retired taxonomy routes are rejected before authentication', async () => {
  const stub = backend('SYS_ADMIN');
  for (const path of [
    'slug-generation',
    'slug-jobs',
    'settings',
    'models',
    tagID + '/labels',
    tagID + '/default-label',
  ]) {
    assert.equal((await forwardTaxonomyManagement(mutation(), path, stub.upstream)).status, 404);
  }
  assert.equal(stub.calls.length, 0);
});

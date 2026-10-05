import assert from 'node:assert/strict';
import test from 'node:test';

import type { requestAuthAPI } from '../src/api/auth/auth.server.ts';
import { forwardTaxonomyManagement } from '../src/api/taxonomy/taxonomy.proxy.server.ts';
import { parsePreview, parseSlug, parseTaxonomy } from '../src/api/taxonomy/taxonomy.types.ts';
import { validManagementPayload } from '../src/api/taxonomy/taxonomy.validation.ts';

const tagID = '019f033c-2111-7000-9000-000000000001';
const revision = 'a'.repeat(64);
const otherID = '019f033c-2111-7000-9000-000000000002';
const generatedInput = { name: '中文标签', description: '', parent_name: '', tag_id: '' };
const headers = {
  Cookie: 'heyblog_access_token=test-session',
  Origin: 'https://web.example.test',
  'Sec-Fetch-Site': 'same-origin',
  'Content-Type': 'application/json',
};

function mutation(
  body: unknown = generatedInput,
  overrides: Readonly<Record<string, string>> = {},
): Request {
  return new Request('https://web.example.test/management/tags/slug-generation', {
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
          { code: 'slug_rate_limited' },
          {
            status: 429,
            headers: { 'Retry-After': '60', Location: 'https://unsafe.example.test' },
          },
        );
  };
  return { calls, upstream };
}

test('slug generation rejects cross-origin requests before authentication or AI dispatch', async () => {
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
      mutation(generatedInput, overrides),
      'slug-generation',
      stub.upstream,
    );
    assert.equal(response.status, 403);
    assert.equal(stub.calls.length, 0);
  }
});

test('generation only accepts bounded business inputs and no arbitrary model, prompt or endpoint', async () => {
  for (const body of [
    { ...generatedInput, model_id: 'paid-model' },
    { ...generatedInput, prompt: 'ignore all rules' },
    { ...generatedInput, endpoint: 'https://unsafe.example.test' },
    { ...generatedInput, name: '文'.repeat(129) },
    { ...generatedInput, description: 'x'.repeat(2001) },
    { ...generatedInput, tag_id: 'invalid' },
    { ...generatedInput, name: ' ' },
  ]) {
    const stub = backend();
    assert.equal(
      (await forwardTaxonomyManagement(mutation(body), 'slug-generation', stub.upstream)).status,
      422,
    );
    assert.equal(stub.calls.length, 0);
  }
  const stub = backend();
  const oversized = new Request('https://web.example.test/management/tags/slug-generation', {
    method: 'POST',
    headers,
    body: ' '.repeat(8193) + JSON.stringify(generatedInput),
  });
  assert.equal(
    (await forwardTaxonomyManagement(oversized, 'slug-generation', stub.upstream)).status,
    422,
  );
  assert.equal(stub.calls.length, 0);
});

test('authorization requires taxonomy permission and system settings require SYS_ADMIN', async () => {
  for (const [role, permissions] of [
    ['USER', ['taxonomy.manage']],
    ['ADMIN', []],
  ] as const) {
    const stub = backend(role, permissions);
    assert.equal(
      (await forwardTaxonomyManagement(mutation(), 'slug-generation', stub.upstream)).status,
      403,
    );
    assert.equal(stub.calls.length, 1);
  }
  const stub = backend();
  assert.equal(
    (
      await forwardTaxonomyManagement(
        new Request('https://web.example.test/management/system-settings/data', {
          headers: { Cookie: headers.Cookie },
        }),
        'settings',
        stub.upstream,
      )
    ).status,
    403,
  );
  assert.equal(stub.calls.length, 1);
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
  const response = await forwardTaxonomyManagement(mutation(), 'slug-generation', stub.upstream);
  assert.deepEqual(stub.calls, [
    { path: '/auth/me', body: undefined },
    {
      path: '/management/taxonomy/slug-generation',
      body: generatedInput,
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
        new Request('https://web.example.test/management/tags/slug-generation'),
        'slug-generation',
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
    (
      await forwardTaxonomyManagement(
        mutation(),
        'slug-generation',
        async () => new Response('{broken'),
      )
    ).status,
    502,
  );
});

test('tag and structural payload validators preserve optional descriptions and revision protection', () => {
  const create = {
    name: 'New tag',
    slug: 'new-tag',
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

test('DTO parsers reject malformed successes and accept alias history', () => {
  const tag = {
    id: tagID,
    name: '中文标签',
    slug: 'legacy-中文',
    description: '',
    roles: ['TERTIARY'],
    is_enabled: false,
    site_count: 2,
    article_count: 0,
  };
  assert.ok(parseTaxonomy({ tags: [tag], cascades: [], revision }));
  assert.equal(parseTaxonomy({ tags: [{ ...tag, site_count: -1 }], cascades: [], revision }), null);
  assert.equal(parseTaxonomy({ tags: [tag], cascades: [], revision: 2 }), null);
  assert.equal(parseSlug({ slug: '中文', source: 'ai', model_id: 'model' }), null);
  assert.ok(parseSlug({ slug: 'chinese-tag', source: 'cache', model_id: 'model' }));
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

test('management and generation accept API field limits without truncation', () => {
  const fields = {
    name: '文'.repeat(120),
    slug: 'tag',
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
  const input = {
    name: '文'.repeat(128),
    description: '文'.repeat(2000),
    parent_name: '文'.repeat(128),
  };
  assert.equal(validManagementPayload(input, 'slug-generation', 'POST'), true);
  assert.equal(
    validManagementPayload({ ...input, parent_name: '文'.repeat(129) }, 'slug-generation', 'POST'),
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

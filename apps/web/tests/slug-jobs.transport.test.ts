import assert from 'node:assert/strict';
import test from 'node:test';

import type { requestAuthAPI } from '../src/api/auth/auth.server.ts';
import { parseSlugJob, parseSlugJobs } from '../src/api/taxonomy/slug-jobs.types.ts';
import { validSlugJobPayload } from '../src/api/taxonomy/slug-jobs.validation.ts';
import { forwardTaxonomyManagement } from '../src/api/taxonomy/taxonomy.proxy.server.ts';
const id = '019f033c-2111-7000-9000-000000000001';
const job = {
  id,
  revision: '1',
  status: 'ready',
  model_id: 'model',
  pause_code: '',
  resume_after: '',
  counts: { total: 1, ready: 1, needs_confirmation: 0, failed: 0, applied: 0 },
  items: [
    {
      tag_id: id,
      name: '生活',
      original_slug: 'legacy-id',
      slug: 'life',
      state: 'ready',
      conflicts: [],
      error_code: '',
      source: 'ai',
    },
  ],
};
const headers = {
  Cookie: 'heyblog_access_token=test',
  Origin: 'https://web.example.test',
  'Sec-Fetch-Site': 'same-origin',
  'Content-Type': 'application/json',
};
test('batch task parsing validates job states, revisions and item identity', () => {
  assert.deepEqual(parseSlugJob(job), job);
  assert.deepEqual(parseSlugJobs({ jobs: [job] }), [job]);
  assert.equal(parseSlugJob({ ...job, revision: 1 }), null);
  assert.equal(parseSlugJob({ ...job, status: 'unknown' }), null);
  assert.equal(parseSlugJob({ ...job, items: [{ ...job.items[0], tag_id: 'bad' }] }), null);
});
test('batch input accepts scoped server selections and rejects arbitrary generation inputs', () => {
  for (const selection of [
    { kind: 'invalid' },
    { kind: 'all' },
    { kind: 'ids', ids: [id] },
    { kind: 'filter', filter: { query: '生活', is_enabled: true } },
  ])
    assert.equal(validSlugJobPayload({ selection }, 'slug-jobs'), true);
  for (const selection of [
    { kind: 'ids', ids: [] },
    { kind: 'ids', ids: Array(501).fill(id) },
    { kind: 'ids', ids: [id, id] },
    { kind: 'all', model_id: 'evil' },
    { kind: 'filter', filter: { prompt: 'evil' } },
  ])
    assert.equal(validSlugJobPayload({ selection }, 'slug-jobs'), false);
  assert.equal(
    validSlugJobPayload({ selection: { kind: 'all' }, endpoint: 'evil' }, 'slug-jobs'),
    false,
  );
  assert.equal(
    validSlugJobPayload(
      { items: [{ tag_id: id, slug: 'life' }], expected_revision: '1' },
      `slug-jobs/${id}/items`,
    ),
    true,
  );
  assert.equal(
    validSlugJobPayload({ tag_ids: [id], expected_revision: '1' }, `slug-jobs/${id}/apply`),
    true,
  );
  assert.equal(validSlugJobPayload({ tag_ids: [id] }, `slug-jobs/${id}/apply`), false);
});
test('batch forwarding authenticates each method and keeps PATCH on its dedicated route', async () => {
  const calls: { path: string; method?: string }[] = [];
  const upstream: typeof requestAuthAPI = async (_request, path, init) => {
    calls.push({ path, method: init?.method });
    return Response.json(
      path === '/auth/me' ? { user: { role: 'ADMIN', permissions: ['taxonomy.manage'] } } : job,
    );
  };
  const path = `slug-jobs/${id}/items`;
  const response = await forwardTaxonomyManagement(
    new Request(`https://web.example.test/management/tags/data/${path}`, {
      method: 'PATCH',
      headers,
      body: JSON.stringify({ items: [{ tag_id: id, slug: 'life' }], expected_revision: '1' }),
    }),
    path,
    upstream,
  );
  assert.equal(response.status, 200);
  assert.deepEqual(calls, [
    { path: '/auth/me', method: undefined },
    { path: `/management/taxonomy/${path}`, method: 'PATCH' },
  ]);
  assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
  for (const [route, method] of [
    [`slug-jobs/${id}/unknown`, 'POST'],
    [`slug-jobs/${id}/apply`, 'PATCH'],
    ['slug-jobs/invalid', 'GET'],
  ]) {
    const blocked = await forwardTaxonomyManagement(
      new Request('https://web.example.test/management/tags/data', { method, headers }),
      route,
      upstream,
    );
    assert.ok([404, 405].includes(blocked.status));
  }
  assert.equal(calls.length, 2);
});
test('batch cross-origin and unauthorized requests never dispatch a paid task', async () => {
  let calls = 0;
  const upstream: typeof requestAuthAPI = async () => {
    calls++;
    return Response.json({ user: { role: 'USER', permissions: [] } });
  };
  const request = (origin: string) =>
    new Request('https://web.example.test/management/tags/data/slug-jobs', {
      method: 'POST',
      headers: { ...headers, Origin: origin },
      body: JSON.stringify({ selection: { kind: 'all' } }),
    });
  assert.equal(
    (await forwardTaxonomyManagement(request('https://evil.example.test'), 'slug-jobs', upstream))
      .status,
    403,
  );
  assert.equal(calls, 0);
  assert.equal(
    (await forwardTaxonomyManagement(request('https://web.example.test'), 'slug-jobs', upstream))
      .status,
    403,
  );
  assert.equal(calls, 1);
});

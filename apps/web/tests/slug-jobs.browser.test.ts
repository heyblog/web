import assert from 'node:assert/strict';
import test from 'node:test';

import {
  applySlugJob,
  controlSlugJob,
  createSlugJob,
  editSlugJob,
  listSlugJobs,
  readSlugJob,
} from '../src/api/taxonomy/slug-jobs.browser.ts';
const id = '019f033c-2111-7000-9000-000000000001';
const job = {
  id,
  status: 'ready',
  revision: '3',
  model_id: 'model',
  pause_code: '',
  resume_after: '',
  counts: { total: 1, ready: 1, failed: 0, applied: 0 },
  items: [
    {
      tag_id: id,
      name: '生活',
      original_slug: 'legacy-id',
      slug: 'life',
      state: 'ready',
      error_code: '',
      source: 'cache',
    },
  ],
};
test('batch browser transport serializes fixed routes, revision checks and selected-only application without retries', async (context) => {
  const calls: { url: string; method: string; body: unknown }[] = [];
  context.mock.method(globalThis, 'fetch', async (input: string, init: RequestInit) => {
    assert.equal(init.credentials, 'same-origin');
    assert.equal(init.redirect, 'error');
    assert.ok(init.signal instanceof AbortSignal);
    calls.push({
      url: input,
      method: init.method ?? 'GET',
      body: typeof init.body === 'string' ? JSON.parse(init.body) : undefined,
    });
    return Response.json(
      input.endsWith('slug-jobs') && init.method === 'GET' ? { jobs: [job] } : job,
    );
  });
  assert.equal((await createSlugJob({ kind: 'ids', ids: [id] })).ok, true);
  assert.equal((await listSlugJobs()).ok, true);
  assert.equal((await readSlugJob(id)).ok, true);
  assert.equal((await controlSlugJob(id, 'pause', '3')).ok, true);
  assert.equal((await editSlugJob(id, '3', [{ tag_id: id, slug: 'life' }])).ok, true);
  assert.equal((await applySlugJob(id, '3', [id])).ok, true);
  const base = '/management/tags/data/slug-jobs';
  assert.deepEqual(calls, [
    { url: base, method: 'POST', body: { selection: { kind: 'ids', ids: [id] } } },
    { url: base, method: 'GET', body: undefined },
    { url: `${base}/${id}`, method: 'GET', body: undefined },
    {
      url: `${base}/${id}/control`,
      method: 'POST',
      body: { action: 'pause', expected_revision: '3' },
    },
    {
      url: `${base}/${id}/items`,
      method: 'PATCH',
      body: { items: [{ tag_id: id, slug: 'life' }], expected_revision: '3' },
    },
    { url: `${base}/${id}/apply`, method: 'POST', body: { tag_ids: [id], expected_revision: '3' } },
  ]);
});
test('poll cancellation reaches fetch and failure is not retried', async (context) => {
  let requests = 0;
  const controller = new AbortController();
  controller.abort();
  context.mock.method(globalThis, 'fetch', async (_input: string, init: RequestInit) => {
    requests++;
    assert.equal(init.signal?.aborted, true);
    throw new DOMException('aborted', 'AbortError');
  });
  assert.deepEqual(await readSlugJob(id, controller.signal), { ok: false, code: 'network_error' });
  assert.equal(requests, 1);
});

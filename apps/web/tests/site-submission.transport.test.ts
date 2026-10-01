import assert from 'node:assert/strict';
import test from 'node:test';

import {
  requestAuditLookup,
  requestAuditReview,
  requestReviewDraft,
  requestSubmissionOptions,
  requestSubmissionSearch,
  requestSubmissionSnapshot,
  submitSite,
} from '../src/api/site-submission/site-submission.browser.ts';
import {
  buildSubmissionPayload,
  emptySubmission,
} from '../src/application/site-submission/site-submission.browser.ts';

test('submission reads preserve dedicated URLs, cancellation, and the untouched response', async (context) => {
  const signal = new AbortController().signal;
  const response = Response.json({ fixture: true });
  const requests: { url: string; init: RequestInit | undefined }[] = [];
  context.mock.method(globalThis, 'fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
    requests.push({ url: String(input), init });
    return response;
  });

  assert.equal(await requestSubmissionOptions(), response);
  assert.equal(await requestSubmissionSnapshot('A/B'), response);
  assert.equal(await requestSubmissionSearch('中文 & 标签', signal), response);

  assert.deepEqual(requests, [
    { url: '/api/site-submissions/options', init: undefined },
    { url: '/api/site-submissions/A%2FB/resolve', init: undefined },
    {
      url: '/api/site-submissions/sites?q=%E4%B8%AD%E6%96%87%20%26%20%E6%A0%87%E7%AD%BE',
      init: { signal },
    },
  ]);
  assert.equal(response.bodyUsed, false);
});

test('audit mutations preserve methods, payloads, and one request per operation', async (context) => {
  const expectations = { expected_site_revision: 7, expected_review_draft_revision: 3 };
  const site = buildSubmissionPayload(emptySubmission(), 'UPDATE').site;
  const decision = {
    ...expectations,
    decision: 'REJECTED' as const,
    reviewer_comment: '固定审核意见',
  };
  const requests: { url: string; init: RequestInit }[] = [];
  context.mock.method(globalThis, 'fetch', async (input: RequestInfo | URL, init: RequestInit) => {
    requests.push({ url: String(input), init });
    return new Response(null, { status: 204 });
  });

  await requestAuditLookup('synthetic-lookup');
  await requestAuditReview('audit-fixture', false, decision);
  await requestAuditReview('audit-fixture', true, expectations);
  await requestReviewDraft('audit-fixture', { ...expectations, site });

  assert.deepEqual(requests, [
    {
      url: '/api/site-submissions/query',
      init: {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ lookup_token: 'synthetic-lookup' }),
      },
    },
    {
      url: '/management/site-submissions/audit-fixture/review',
      init: {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(decision),
      },
    },
    {
      url: '/management/site-submissions/audit-fixture/review-draft',
      init: {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(expectations),
      },
    },
    {
      url: '/management/site-submissions/audit-fixture/review-draft',
      init: {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...expectations, site }),
      },
    },
  ]);
});

test('submission transport receives the mapped payload without reading editable state', async () => {
  const payload = buildSubmissionPayload(emptySubmission(), 'CREATE');
  const signal = new AbortController().signal;
  let requests = 0;
  const response = await submitSite(
    { action: 'CREATE', siteShortID: '', payload },
    {
      signal,
      fetch: async (input, init) => {
        requests += 1;
        assert.equal(input, '/api/site-submissions/create');
        assert.deepEqual(init, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
          signal,
        });
        return new Response(null, { status: 201 });
      },
    },
  );
  assert.equal(response.status, 201);
  assert.equal(requests, 1);
});

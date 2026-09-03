import assert from 'node:assert/strict';
import test from 'node:test';

import { forwardSiteSubmission } from '../src/application/site-submission/proxy.server.ts';

const configuration = {
  apiBaseUrl: 'http://api.internal:10201',
  apiWebToken: 'test-web-service-token-0123456789abcdef',
};

test('forwards a same-origin JSON mutation through the authenticated service boundary', async () => {
  let upstreamRequest: { url: string; init?: RequestInit } | undefined;
  const response = await forwardSiteSubmission(
    browserRequest('POST', '{}', 'application/json; charset=utf-8'),
    '/site-submissions',
    'POST',
    {
      loadConfig: () => configuration,
      fetch: async (input, init) => {
        upstreamRequest = { url: input.toString(), init };
        return Response.json({ audit_id: 'audit-id' }, { status: 201 });
      },
    },
  );

  assert.equal(response.status, 201);
  assert.deepEqual(await response.json(), { audit_id: 'audit-id' });
  assert.equal(upstreamRequest?.url, 'http://api.internal:10201/site-submissions');
  const headers = new Headers(upstreamRequest?.init?.headers);
  assert.equal(headers.get('X-HeyBlog-Web-Token'), configuration.apiWebToken);
  assert.equal(headers.get('Content-Type'), 'application/json');
  assert.equal(headers.get('X-Request-ID'), 'browser-request.123');
  assert.equal(headers.get('X-Real-IP'), '203.0.113.13');
  assert.equal(headers.get('X-Forwarded-For'), '203.0.113.13');
});

test('rejects cross-origin and unsupported-media requests before upstream access', async () => {
  let fetchCalls = 0;
  const dependencies = {
    loadConfig: () => configuration,
    fetch: async () => {
      fetchCalls += 1;
      return Response.json({});
    },
  };
  const crossOrigin = browserRequest('POST', '{}', 'application/json');
  crossOrigin.headers.set('Sec-Fetch-Site', 'cross-site');

  const forbidden = await forwardSiteSubmission(
    crossOrigin,
    '/site-submissions',
    'POST',
    dependencies,
  );
  const unsupported = await forwardSiteSubmission(
    browserRequest('POST', '{}', 'text/plain'),
    '/site-submissions',
    'POST',
    dependencies,
  );

  assert.equal(forbidden.status, 403);
  assert.equal(unsupported.status, 415);
  assert.equal(fetchCalls, 0);
});

test('rejects oversized submission bodies before upstream access', async () => {
  let fetchCalled = false;

  const response = await forwardSiteSubmission(
    browserRequest('POST', 'x'.repeat(256_001), 'application/json'),
    '/site-submissions',
    'POST',
    {
      loadConfig: () => configuration,
      fetch: async () => {
        fetchCalled = true;
        return Response.json({});
      },
    },
  );

  assert.equal(response.status, 413);
  assert.equal(fetchCalled, false);
});

function browserRequest(method: 'GET' | 'POST', body?: string, contentType?: string): Request {
  const headers = new Headers({
    'Sec-Fetch-Site': 'same-origin',
    'Sec-Fetch-Mode': 'cors',
    'Sec-Fetch-Dest': 'empty',
    'X-Request-ID': 'browser-request.123',
    'X-Real-IP': '203.0.113.13',
  });
  if (contentType) headers.set('Content-Type', contentType);
  return new Request('https://web.example.test/api/site-submissions/create', {
    method,
    headers,
    body,
  });
}

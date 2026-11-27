import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { test } from 'node:test';

import { createApp } from '../src/app.ts';
import type { Adapter, EdgeRequest } from '../src/protocol.ts';

const token = 'test-service-token-at-least-32-characters';
const request: EdgeRequest = {
  url: 'https://example.com/',
  method: 'GET',
  platform: 'CF',
  headers: {},
  maxBytes: 512,
  timeoutMs: 1000,
  resolvedIP: '1.1.1.1',
};
function dispatch(input: EdgeRequest, adapter: Adapter, body = JSON.stringify(input)) {
  return createApp(adapter).request('/v1/execute', {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body,
  });
}

test('configured Unicode headers reach the HTTP target as UTF-8 bytes', async (t) => {
  // Given configured text accepted by Go, including multibyte and Latin-1 characters.
  const text = 'HeyBlog 检测 Café';
  const expected = Buffer.from(text, 'utf8').toString('latin1');
  const observed: string[] = [];
  const server = createServer((incoming, response) => {
    observed.push(incoming.headers['user-agent'] ?? '');
    observed.push(String(incoming.headers['x-description']));
    response.end('ok');
  });
  t.after(async () => {
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  // When either adapter sends the prepared headers through native Fetch.
  for (const platform of ['CF', 'EO'] as const) {
    for (const headerEncoding of ['utf8', 'latin1'] as const) {
      const value = headerEncoding === 'utf8' ? text : expected;
      const response = await dispatch(
        {
          ...request,
          platform,
          headerEncoding,
          headers: { 'User-Agent': value, 'X-Description': value },
        },
        {
          platform,
          token,
          node: {},
          fetch: (_, init) => fetch(`http://127.0.0.1:${address.port}/`, init),
        },
      );
      // Then the executor succeeds instead of reporting a platform error before fetch.
      assert.equal(response.status, 200);
    }
  }
  assert.deepEqual(
    observed,
    Array.from({ length: 8 }, () => expected),
  );
});

test('opaque validator bytes are preserved in the Go wire encoding', async () => {
  // Given an ETag containing a byte that is not valid UTF-8 by itself.
  const etag = '"\xe9"';
  // When Go resends the validator using the byte-preserving encoding.
  const response = await dispatch(
    { ...request, headerEncoding: 'latin1', headers: { 'If-None-Match': etag } },
    {
      platform: 'CF',
      token,
      node: {},
      fetch: async (_, init) => {
        assert.equal(new Headers(init.headers).get('If-None-Match'), etag);
        return new Response(null, { status: 304, headers: { etag } });
      },
    },
  );
  // Then the response still supplies the same opaque validator to Go.
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), { status: 304, headers: { etag }, body: '', node: {} });
});

test('invalid byte strings and control characters are rejected before fetch', async () => {
  // Given malformed raw-byte values or HTTP control characters.
  let calls = 0;
  for (const value of ['中文', 'invalid\x01value', 'invalid\x7fvalue']) {
    // When they cross the authenticated request boundary.
    const response = await dispatch(
      { ...request, headerEncoding: 'latin1', headers: { 'User-Agent': value } },
      {
        platform: 'CF',
        token,
        node: {},
        fetch: async () => {
          calls++;
          return new Response();
        },
      },
    );
    // Then the executor reports the request boundary failure without dispatching.
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), {
      status: 0,
      headers: {},
      body: '',
      node: {},
      error: 'unsafe_target',
    });
  }
  assert.equal(calls, 0);
});

test('Go HTML escaping does not reject a valid maximum-size header configuration', async () => {
  // Given sixteen 1024-byte values accepted by task configuration validation.
  const headers = Object.fromEntries(
    Array.from({ length: 16 }, (_, index) => [`X-Test-${index}`, '<'.repeat(1024)]),
  );
  const input = { ...request, headers };
  const body = JSON.stringify(input).replaceAll('<', '\\u003c');
  let calls = 0;
  // When Go's default JSON escaping expands the transport envelope beyond 64 KiB.
  const response = await dispatch(
    input,
    {
      platform: 'CF',
      token,
      node: {},
      fetch: async (_, init) => {
        calls++;
        assert.equal(new Headers(init.headers).get('X-Test-15'), '<'.repeat(1024));
        return new Response('ok');
      },
    },
    body,
  );
  // Then the bounded executor accepts the still-valid task request.
  assert.equal(response.status, 200);
  assert.equal(calls, 1);
});

test('oversized execution envelopes are rejected before fetch', async () => {
  // Given an authenticated body beyond the executor's transport allowance.
  let calls = 0;
  // When a caller exceeds the envelope cap.
  const response = await dispatch(
    request,
    {
      platform: 'CF',
      token,
      node: {},
      fetch: async () => {
        calls++;
        return new Response('unexpected');
      },
    },
    ' '.repeat(256 * 1024 + 1),
  );
  // Then no target request is sent.
  assert.equal(response.status, 413);
  assert.equal(calls, 0);
});

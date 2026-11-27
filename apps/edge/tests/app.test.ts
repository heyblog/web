import assert from 'node:assert/strict';
import { test } from 'node:test';

import { createApp } from '../src/app.ts';
import cloudflare from '../src/cloudflare.ts';
import { edgeOneFetch } from '../src/edgeone-fetch.ts';
import { execute } from '../src/execute.ts';
import { type Adapter, type EdgeRequest } from '../src/protocol.ts';
import { publicIP } from '../src/target.ts';

const token = 'service-token-with-at-least-32-characters';
const request: EdgeRequest = {
  url: 'https://example.com/',
  method: 'GET',
  platform: 'CF',
  headers: { 'User-Agent': 'Configured-UA', 'X-HeyBlog-Proof': 'request-proof' },
  maxBytes: 512,
  timeoutMs: 1000,
  resolvedIP: '1.1.1.1',
};
test('EdgeOne pins validated DNS through its documented eo extension', async () => {
  const target = { ...request, platform: 'EO' as const, resolvedIP: '2606:4700:4700::1111' };
  let calls = 0;
  const result = await execute(target, {
    platform: 'EO',
    token,
    node: {},
    fetch: edgeOneFetch('https://edge-eo.example.com/v1/execute', async (url, options) => {
      calls++;
      assert.equal(url, target.url);
      assert.equal(options.eo.resolveOverride, '2606:4700:4700::1111');
      assert.deepEqual(options.eo.timeoutSetting, {
        connectTimeout: 1000,
        readTimeout: 1000,
        writeTimeout: 1000,
      });
      assert.equal(options.redirect, 'manual');
      assert.equal(options.cache, undefined);
      assert.ok(options.signal instanceof AbortSignal);
      return new Response(null, { status: 302, headers: { location: '/next' } });
    }),
  });
  assert.equal(result.status, 302);
  assert.equal(calls, 1);
});

test('EdgeOne rejects ingress-host targets before native fetch', async () => {
  // A same-host fetch may use CDN origin routing instead of the checked IP.
  const ingress = 'https://EDGE-EO.example.com/v1/execute';
  let calls = 0;
  const app = createApp({
    platform: 'EO',
    token,
    node: {},
    fetch: edgeOneFetch(ingress, async () => {
      calls++;
      return new Response('unexpected');
    }),
  });
  for (const url of ['https://edge-eo.example.com/blog', 'http://edge-eo.example.com./blog']) {
    const response = await app.request(ingress, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...request, platform: 'EO', url }),
    });
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

test('EdgeOne fetch events authenticate with the runtime secret', async (t) => {
  type FetchEvent = {
    request: Request;
    respondWith(response: Response | Promise<Response>): void;
  };
  let listener: ((event: FetchEvent) => void) | undefined;
  const runtime: { EDGE_SERVICE_TOKEN?: string } = {};
  Object.defineProperties(globalThis, {
    addEventListener: {
      value: (_type: string, handler: (event: FetchEvent) => void) => {
        listener = handler;
      },
      configurable: true,
    },
    env: { value: runtime, configurable: true },
  });
  t.after(() => {
    Reflect.deleteProperty(globalThis, 'addEventListener');
    Reflect.deleteProperty(globalThis, 'env');
  });
  let calls = 0;
  t.mock.method(globalThis, 'fetch', async () => {
    calls++;
    return new Response(null, { status: 204 });
  });
  await import('../src/edgeone.ts');

  function dispatch(): Promise<Response> {
    const handle = listener;
    assert.ok(handle);
    return new Promise((resolve) => {
      handle({
        request: new Request('https://edge-eo.example.com/v1/execute', {
          method: 'POST',
          headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
          body: JSON.stringify({ ...request, platform: 'EO' }),
        }),
        respondWith: resolve,
      });
    });
  }

  assert.equal((await dispatch()).status, 401);
  assert.equal(calls, 0);

  runtime.EDGE_SERVICE_TOKEN = token;
  const response = await dispatch();
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), { status: 204, headers: {}, body: '', node: {} });
  assert.equal(calls, 1);
});

function adapter(fetch: Adapter['fetch']): Adapter {
  return { platform: 'CF', token, node: { colo: 'test' }, fetch };
}

test('unauthenticated callers cannot execute target requests', async () => {
  // Given a private executor.
  let calls = 0;
  const app = createApp(
    adapter(async () => {
      calls++;
      return new Response('unexpected');
    }),
  );
  // When a request has no service credential.
  const response = await app.request('/v1/execute', {
    method: 'POST',
    body: JSON.stringify(request),
  });
  // Then the target is untouched.
  assert.equal(response.status, 401);
  assert.equal(calls, 0);
});
test('Cloudflare bypasses cache and preserves redirects without forwarding credentials', async (t) => {
  // Given a target that redirects.
  let calls = 0;
  t.mock.method(globalThis, 'fetch', async (_: string, init: RequestInit) => {
    calls++;
    assert.equal(init.redirect, 'manual');
    assert.equal(init.cache, 'no-store');
    const headers = new Headers(init.headers);
    assert.equal(headers.get('Cache-Control'), 'no-cache, no-store');
    assert.equal(headers.get('Pragma'), 'no-cache');
    assert.equal(headers.get('Authorization'), null);
    assert.equal(headers.get('User-Agent'), 'Configured-UA');
    assert.equal(headers.get('X-HeyBlog-Proof'), 'request-proof');
    return new Response(null, {
      status: 302,
      headers: { Location: 'http://127.0.0.1/', 'Set-Cookie': 'private=value' },
    });
  });
  // When the authenticated API dispatches one request.
  const ingress = new Request('https://edge-cf.example.com/v1/execute', {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
  const response = await cloudflare.fetch(Object.assign(ingress, { cf: { colo: 'test' } }), {
    EDGE_SERVICE_TOKEN: token,
  });
  // Then Go receives the redirect and must validate the next hop itself.
  assert.equal(response.status, 200);
  assert.equal(calls, 1);
  assert.deepEqual(await response.json(), {
    status: 302,
    headers: { location: 'http://127.0.0.1/' },
    body: '',
    node: { colo: 'test' },
  });
});
test('private literal targets are rejected before native fetch', async () => {
  // Given a public DNS address attached to a malicious literal URL.
  let calls = 0;
  // When the edge independently checks the request boundary.
  const response = await execute(
    { ...request, url: 'http://[::ffff:7f00:1]/' },
    adapter(async () => {
      calls++;
      return new Response();
    }),
  );
  // Then caller-supplied resolution cannot bypass literal validation.
  assert.equal(response.error, 'unsafe_target');
  assert.equal(calls, 0);
});
test('oversized response streams are cancelled at the limit', async () => {
  // Given a streaming target that could continue indefinitely.
  let cancelled = false;
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(new Uint8Array(513));
    },
    cancel() {
      cancelled = true;
    },
  });
  // When the response exceeds its 512-byte contract.
  const response = await execute(
    request,
    adapter(async () => new Response(stream)),
  );
  // Then no partial content is treated as a valid business result.
  assert.equal(response.error, 'response_limit');
  assert.equal(cancelled, true);
});
test('conditional not-modified responses remain valid without content', async () => {
  // Given a target's conditional request response.
  // When a 304 is returned.
  const response = await execute(
    request,
    adapter(async () => new Response(null, { status: 304, headers: { ETag: 'unchanged' } })),
  );
  // Then the transport preserves it for the Go RSS parser.
  assert.equal(response.status, 304);
  assert.equal(response.body, '');
  assert.equal(response.error, undefined);
});
test('special-use IP ranges are excluded for both platforms', () => {
  // Given reserved and transition addresses.
  const addresses = [
    '127.0.0.1',
    '169.254.169.254',
    '100.64.0.1',
    '10.0.0.1',
    '192.0.2.1',
    '198.18.0.1',
    '203.0.113.1',
    '::1',
    '::ffff:127.0.0.1',
    '2001:db8::1',
    '2001:2::1',
    '2002:7f00:1::',
    'fc00::1',
  ];
  // When classifying targets.
  const accepted = addresses.filter(publicIP);
  // Then none can become a pinned destination.
  assert.deepEqual(accepted, []);
  assert.equal(publicIP('1.1.1.1'), true);
  assert.equal(publicIP('2606:4700:4700::1111'), true);
});
test('task headers cannot carry service authentication', async () => {
  // Given an authorization header smuggled into the task configuration.
  let calls = 0;
  // When preparing the target request.
  const response = await execute(
    { ...request, headers: { Authorization: 'Bearer private-service-token' } },
    adapter(async () => {
      calls++;
      return new Response();
    }),
  );
  // Then the sensitive header is rejected before native fetch.
  assert.equal(response.error, 'unsafe_target');
  assert.equal(calls, 0);
});

test('signed targets requiring WHATWG rewrites never reach either platform', async () => {
  // Given redirect destinations whose serialized URL differs from the signed value.
  const urls = [
    'https://example.com/a/%2e%2e/secret',
    'https://example.com:443/a',
    'https://EXAMPLE.com/a',
    'https://example.com/a/../secret',
  ];
  let calls = 0;
  for (const platform of ['CF', 'EO'] as const) {
    for (const url of urls) {
      // When the shared edge boundary validates the next signed hop.
      const result = await execute(
        { ...request, platform, url },
        {
          platform,
          token,
          node: {},
          fetch: async () => {
            calls++;
            return new Response('unexpected');
          },
        },
      );
      // Then it fails closed instead of attaching the proof to a different request URL.
      assert.equal(result.error, 'unsafe_target', `${platform}: ${url}`);
    }
  }
  assert.equal(calls, 0);
});

test('canonical escaped targets preserve the signed URL on either platform', async () => {
  // Given a normalized Go-style URL with escaped path/query data.
  const url = 'https://example.com/blog/a%20b?next=%2Fhome&tag=%E4%B8%AD';
  for (const platform of ['CF', 'EO'] as const) {
    let observed = '';
    // When a signed hop enters native Fetch.
    const result = await execute(
      { ...request, platform, url },
      {
        platform,
        token,
        node: {},
        fetch: async (target, init) => {
          observed = new Request(target.url, init).url;
          return new Response('ok');
        },
      },
    );
    // Then normalization leaves the signed request identity intact.
    assert.equal(result.status, 200);
    assert.equal(observed, url);
  }
});

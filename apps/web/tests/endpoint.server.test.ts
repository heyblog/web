import assert from 'node:assert/strict';
import test from 'node:test';

import {
  type ApiEndpointPolicy,
  apiWebTokenHeader,
  createApiEndpoint,
  handleApiRequest,
} from '../src/application/api/endpoint.server.ts';

const policy = {
  audience: 'web-only',
  method: 'GET',
  upstreamPath: '/ping',
  responseHeaders: ['content-type'],
} satisfies ApiEndpointPolicy;

const configuration = {
  apiBaseUrl: 'http://api.internal:10201',
  apiWebToken: 'test-web-service-token-0123456789abcdef',
};

test('forwards an allowed page fetch with only declared headers', async () => {
  let upstreamRequest: { url: string; init?: RequestInit } | undefined;
  const response = await handleApiRequest(pageRequest(), policy, {
    loadConfig: () => configuration,
    fetch: async (input, init) => {
      upstreamRequest = { url: input.toString(), init };
      return Response.json(
        { message: 'pong' },
        {
          headers: {
            'Content-Encoding': 'gzip',
            'Content-Length': '999',
            Location: 'http://api.internal:10201/private',
            'X-Request-ID': 'upstream-request.123',
          },
        },
      );
    },
  });

  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), { message: 'pong' });
  assert.equal(upstreamRequest?.url, 'http://api.internal:10201/ping');
  const headers = new Headers(upstreamRequest?.init?.headers);
  assert.equal(headers.get(apiWebTokenHeader), configuration.apiWebToken);
  assert.equal(headers.get('X-Real-IP'), '203.0.113.12');
  assert.equal(headers.get('X-Forwarded-For'), '203.0.113.12');
  assert.equal(headers.get('Cookie'), null);
  assert.equal(response.headers.get('Content-Encoding'), null);
  assert.equal(response.headers.get('Content-Length'), null);
  assert.equal(response.headers.get('Location'), null);
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
  assert.equal(response.headers.get('X-Request-ID'), 'upstream-request.123');
});

test('forwards only explicitly declared authentication state', async () => {
  let upstreamHeaders: Headers | undefined;
  const response = await handleApiRequest(
    pageRequest('https://web.example.test/api/ping', {
      headers: {
        Authorization: 'Bearer browser-session',
        Cookie: 'session=browser-session',
      },
    }),
    {
      ...policy,
      requestHeaders: ['authorization', 'cookie'],
      forwardSetCookie: true,
    },
    {
      loadConfig: () => configuration,
      fetch: async (_input, init) => {
        upstreamHeaders = new Headers(init?.headers);
        return new Response('{"message":"pong"}', {
          headers: [
            ['Content-Type', 'application/json'],
            ['Set-Cookie', 'session=renewed; Path=/; HttpOnly; SameSite=Lax'],
            ['Set-Cookie', 'csrf=rotated; Path=/; Secure; SameSite=Strict'],
          ],
        });
      },
    },
  );

  assert.equal(upstreamHeaders?.get('Authorization'), 'Bearer browser-session');
  assert.equal(upstreamHeaders?.get('Cookie'), 'session=browser-session');
  assert.equal(upstreamHeaders?.get(apiWebTokenHeader), configuration.apiWebToken);
  assert.deepEqual(response.headers.getSetCookie(), [
    'session=renewed; Path=/; HttpOnly; SameSite=Lax',
    'csrf=rotated; Path=/; Secure; SameSite=Strict',
  ]);
});

test('rejects direct navigation and requests without Fetch Metadata', async () => {
  let fetchCalled = false;
  const dependencies = {
    loadConfig: () => configuration,
    fetch: async () => {
      fetchCalled = true;
      return Response.json({ message: 'pong' });
    },
  };

  for (const request of [
    new Request('https://web.example.test/api/ping', {
      headers: {
        'Sec-Fetch-Site': 'same-origin',
        'Sec-Fetch-Mode': 'navigate',
        'Sec-Fetch-Dest': 'document',
      },
    }),
    new Request('https://web.example.test/api/ping'),
  ]) {
    const response = await handleApiRequest(request, policy, dependencies);
    assert.equal(response.status, 403);
    assert.equal(response.headers.get('Content-Type'), 'application/problem+json');
  }
  assert.equal(fetchCalled, false);
});

test('rejects undeclared query fields before calling upstream', async () => {
  let fetchCalled = false;
  const response = await handleApiRequest(
    pageRequest('https://web.example.test/api/ping?redirect=http://private.test'),
    policy,
    {
      loadConfig: () => configuration,
      fetch: async () => {
        fetchCalled = true;
        return Response.json({ message: 'pong' });
      },
    },
  );

  assert.equal(response.status, 400);
  assert.equal(fetchCalled, false);
});

test('forwards declared query fields without changing their values', async () => {
  let upstreamURL: string | undefined;
  const response = await handleApiRequest(
    pageRequest('https://web.example.test/api/ping?tag=first&tag=second'),
    { ...policy, queryParameters: ['tag'] },
    {
      loadConfig: () => configuration,
      fetch: async (input) => {
        upstreamURL = input.toString();
        return Response.json({ message: 'pong' });
      },
    },
  );

  assert.equal(response.status, 200);
  assert.equal(upstreamURL, 'http://api.internal:10201/ping?tag=first&tag=second');
});

test('preserves upstream status and body', async () => {
  const response = await handleApiRequest(pageRequest(), policy, {
    loadConfig: () => configuration,
    fetch: async () => Response.json({ code: 'service_unavailable' }, { status: 503 }),
  });

  assert.equal(response.status, 503);
  assert.deepEqual(await response.json(), { code: 'service_unavailable' });
});

test('maps connection failures to a safe bad gateway problem', async () => {
  const response = await handleApiRequest(pageRequest(), policy, {
    loadConfig: () => configuration,
    fetch: async () => {
      throw new Error('connect http://api.internal:10201 secret');
    },
  });
  const body = await response.text();

  assert.equal(response.status, 502);
  assert.match(body, /"code":"bad_gateway"/);
  assert.doesNotMatch(body, /api\.internal|secret/);
});

test('aborts an upstream request at the route timeout', async () => {
  const response = await handleApiRequest(
    pageRequest(),
    { ...policy, timeoutMs: 1 },
    {
      loadConfig: () => configuration,
      fetch: async (_input, init) =>
        await new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener('abort', () => reject(new Error('aborted')), {
            once: true,
          });
        }),
    },
  );

  assert.equal(response.status, 502);
});

test('propagates browser cancellation to the upstream request', async () => {
  const controller = new AbortController();
  let upstreamSignal: AbortSignal | undefined;
  const responsePromise = handleApiRequest(
    pageRequest('https://web.example.test/api/ping', { signal: controller.signal }),
    policy,
    {
      loadConfig: () => configuration,
      fetch: async (_input, init) =>
        await new Promise<Response>((_resolve, reject) => {
          upstreamSignal = init?.signal ?? undefined;
          upstreamSignal?.addEventListener('abort', () => reject(upstreamSignal?.reason), {
            once: true,
          });
        }),
    },
  );

  controller.abort(new Error('browser disconnected'));
  const response = await responsePromise;

  assert.equal(upstreamSignal?.aborted, true);
  assert.equal(response.status, 502);
});

test('public policy permits non-browser clients while retaining explicit CORS', async () => {
  const publicPolicy = {
    ...policy,
    audience: 'public',
    cors: { allowOrigins: ['https://client.example.test'] },
  } satisfies ApiEndpointPolicy;
  const endpoint = createApiEndpoint(publicPolicy, {
    loadConfig: () => configuration,
    fetch: async () => Response.json({ message: 'pong' }),
  });

  const response = await endpoint.GET({
    request: new Request('https://web.example.test/api/ping', {
      headers: { Origin: 'https://client.example.test' },
    }),
  } as Parameters<typeof endpoint.GET>[0]);

  assert.equal(response.status, 200);
  assert.equal(response.headers.get('Access-Control-Allow-Origin'), 'https://client.example.test');

  const preflight = await endpoint.OPTIONS({
    request: new Request('https://web.example.test/api/ping', {
      method: 'OPTIONS',
      headers: {
        Origin: 'https://client.example.test',
        'Access-Control-Request-Method': 'GET',
      },
    }),
  } as Parameters<typeof endpoint.OPTIONS>[0]);
  assert.equal(preflight.status, 204);
  assert.equal(preflight.headers.get('Access-Control-Allow-Methods'), 'GET');
});

test('public policy rejects undeclared origins and preflight headers', async () => {
  let fetchCalled = false;
  const publicPolicy = {
    ...policy,
    audience: 'public',
    cors: {
      allowOrigins: ['https://client.example.test'],
      allowHeaders: ['authorization'],
    },
  } satisfies ApiEndpointPolicy;
  const endpoint = createApiEndpoint(publicPolicy, {
    loadConfig: () => configuration,
    fetch: async () => {
      fetchCalled = true;
      return Response.json({ message: 'pong' });
    },
  });

  const disallowedOrigin = await endpoint.GET({
    request: new Request('https://web.example.test/api/ping', {
      headers: { Origin: 'https://attacker.example.test' },
    }),
  } as Parameters<typeof endpoint.GET>[0]);
  const disallowedHeader = await endpoint.OPTIONS({
    request: new Request('https://web.example.test/api/ping', {
      method: 'OPTIONS',
      headers: {
        Origin: 'https://client.example.test',
        'Access-Control-Request-Method': 'GET',
        'Access-Control-Request-Headers': 'X-Internal-Token',
      },
    }),
  } as Parameters<typeof endpoint.OPTIONS>[0]);

  assert.equal(disallowedOrigin.status, 403);
  assert.equal(disallowedHeader.status, 403);
  assert.equal(fetchCalled, false);
});

function pageRequest(url = 'https://web.example.test/api/ping', init: RequestInit = {}): Request {
  const headers = new Headers(init.headers);
  headers.set('X-Real-IP', '203.0.113.12');
  headers.set('Sec-Fetch-Site', 'same-origin');
  headers.set('Sec-Fetch-Mode', 'cors');
  headers.set('Sec-Fetch-Dest', 'empty');
  return new Request(url, {
    ...init,
    headers,
  });
}

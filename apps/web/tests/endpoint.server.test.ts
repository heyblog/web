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
  assert.equal(headers.get('Cookie'), null);
  assert.equal(response.headers.get('Content-Encoding'), null);
  assert.equal(response.headers.get('Content-Length'), null);
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
  assert.equal(response.headers.get('X-Request-ID'), 'upstream-request.123');
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

function pageRequest(url = 'https://web.example.test/api/ping'): Request {
  return new Request(url, {
    headers: {
      'Sec-Fetch-Site': 'same-origin',
      'Sec-Fetch-Mode': 'cors',
      'Sec-Fetch-Dest': 'empty',
    },
  });
}

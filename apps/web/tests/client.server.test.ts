import assert from 'node:assert/strict';
import test from 'node:test';

import { fetchApiJson } from '../src/application/api/client.server.ts';
import { apiWebTokenHeader } from '../src/application/api/endpoint.server.ts';

const configuration = {
  apiBaseUrl: 'http://api.internal:10201',
  apiWebToken: 'test-web-service-token-0123456789abcdef',
};

test('fetches JSON through the authenticated Web service boundary', async () => {
  let upstreamRequest: { url: string; init?: RequestInit } | undefined;
  const result = await fetchApiJson<{ siteCount: number }>('/home', {
    request: new Request('https://web.example.test/', {
      headers: { 'X-Real-IP': '203.0.113.11' },
    }),
    loadConfig: () => configuration,
    fetch: async (input, init) => {
      upstreamRequest = { url: input.toString(), init };
      return Response.json({ siteCount: 12 });
    },
  });

  assert.deepEqual(result, { kind: 'success', data: { siteCount: 12 } });
  assert.equal(upstreamRequest?.url, 'http://api.internal:10201/home');
  assert.equal(upstreamRequest?.init?.method, 'GET');
  assert.equal(upstreamRequest?.init?.redirect, 'error');
  const headers = new Headers(upstreamRequest?.init?.headers);
  assert.equal(headers.get('Accept'), 'application/json');
  assert.equal(headers.get(apiWebTokenHeader), configuration.apiWebToken);
  assert.equal(headers.get('X-Real-IP'), '203.0.113.11');
  assert.equal(headers.get('X-Forwarded-For'), '203.0.113.11');
});

test('maps accepted API statuses without exposing upstream details', async () => {
  const cases = [
    { status: 400, kind: 'bad-request' },
    { status: 404, kind: 'not-found' },
    { status: 401, kind: 'unavailable' },
    { status: 503, kind: 'unavailable' },
  ] as const;

  for (const testCase of cases) {
    const result = await fetchApiJson('/home', {
      loadConfig: () => configuration,
      fetch: async () => new Response('upstream detail', { status: testCase.status }),
    });
    assert.deepEqual(result, { kind: testCase.kind });
  }
});

test('maps invalid JSON and invalid server configuration to unavailable', async () => {
  const invalidJSON = await fetchApiJson('/home', {
    loadConfig: () => configuration,
    fetch: async () => new Response('{invalid', { status: 200 }),
  });
  const invalidConfiguration = await fetchApiJson('/home', {
    loadConfig: () => {
      throw new Error('configuration unavailable');
    },
  });

  assert.deepEqual(invalidJSON, { kind: 'unavailable' });
  assert.deepEqual(invalidConfiguration, { kind: 'unavailable' });
});

test('maps an API timeout to unavailable', async () => {
  const result = await fetchApiJson('/home', {
    loadConfig: () => configuration,
    timeoutMs: 1,
    fetch: async (_input, init) =>
      await new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener('abort', () => reject(new Error('aborted')), {
          once: true,
        });
      }),
  });

  assert.deepEqual(result, { kind: 'unavailable' });
});

test('rejects paths that can escape or alter the declared upstream route', async () => {
  let configurationLoads = 0;
  const loadConfig = () => {
    configurationLoads += 1;
    return configuration;
  };

  for (const path of [
    'home',
    '//attacker.example.test/home',
    '/\\attacker.example.test',
    '/home#private',
  ]) {
    await assert.rejects(
      fetchApiJson(path as `/${string}`, { loadConfig }),
      /API path must be an absolute path without a fragment/,
    );
  }
  assert.equal(configurationLoads, 0);
});

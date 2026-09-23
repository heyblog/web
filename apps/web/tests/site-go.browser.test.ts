import assert from 'node:assert/strict';
import test from 'node:test';

import { handleApiRequest } from '../src/application/api/endpoint.server.ts';
import { copySiteGoLink, previewRandomSite } from '../src/application/site-go/site-go.browser.ts';

test('copy reports success, denial and unsupported clipboard without throwing', async () => {
  let copied = '';
  assert.equal(
    await copySiteGoLink('https://www.heyblog.net/site/go', {
      writeText: async (text) => {
        copied = text;
      },
    }),
    true,
  );
  assert.equal(copied, 'https://www.heyblog.net/site/go');
  assert.equal(await copySiteGoLink(copied, undefined), false);
  assert.equal(
    await copySiteGoLink(copied, {
      writeText: async () => {
        throw new Error('denied');
      },
    }),
    false,
  );
});

test('browser preview uses same-origin random endpoint with no-store and cancellation', async () => {
  const signal = new AbortController().signal;
  const selection = await previewRandomSite('技术', '', signal, async (input, init) => {
    assert.equal(String(input), '/api/site-random?level1=%E6%8A%80%E6%9C%AF');
    assert.equal(init?.cache, 'no-store');
    assert.equal(init?.signal, signal);
    return Response.json({ site: null });
  });
  assert.deepEqual(selection, { kind: 'success', site: null });
});

test('browser errors expose curated text only and distinguish invalid selection from outages', async () => {
  const signal = new AbortController().signal;
  const invalid = await previewRandomSite('技术', '旅行', signal, async () =>
    Response.json({ code: 'random_classification_mismatch', detail: 'private' }, { status: 400 }),
  );
  assert.equal(invalid.kind, 'error');
  if (invalid.kind === 'error') assert.match(invalid.message, /不属于/u);
  for (const response of [new Response('private', { status: 500 }), new Response('bad json')]) {
    const failure = await previewRandomSite('', '', signal, async () => response);
    assert.deepEqual(failure, { kind: 'error', message: '暂时无法获取博客，请稍后重试。' });
  }
});

test('random proxy rejects cross-site and unsupported fields, preserves API errors', async () => {
  const policy = {
    audience: 'web-only',
    method: 'GET',
    upstreamPath: '/sites/random',
    queryParameters: ['level1', 'level2'],
    responseHeaders: ['content-type'],
  } as const;
  const headers = {
    'Sec-Fetch-Site': 'same-origin',
    'Sec-Fetch-Mode': 'cors',
    'Sec-Fetch-Dest': 'empty',
  };
  const dependencies = {
    loadConfig: () => ({ apiBaseUrl: 'http://api.internal:10201', apiWebToken: 'test-token' }),
    fetch: async (input: string | URL | Request) => {
      assert.equal(
        String(input),
        'http://api.internal:10201/sites/random?level1=%E6%8A%80%E6%9C%AF&level1=%E7%94%9F%E6%B4%BB',
      );
      return Response.json({ code: 'random_duplicate_parameter' }, { status: 400 });
    },
  };
  assert.equal(
    (
      await handleApiRequest(
        new Request('https://www.heyblog.net/api/site-random'),
        policy,
        dependencies,
      )
    ).status,
    403,
  );
  assert.equal(
    (
      await handleApiRequest(
        new Request('https://www.heyblog.net/api/site-random?preview=true', { headers }),
        policy,
        dependencies,
      )
    ).status,
    400,
  );
  const response = await handleApiRequest(
    new Request('https://www.heyblog.net/api/site-random?level1=技术&level1=生活', { headers }),
    policy,
    dependencies,
  );
  assert.equal(response.status, 400);
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
  assert.deepEqual(await response.json(), { code: 'random_duplicate_parameter' });
});

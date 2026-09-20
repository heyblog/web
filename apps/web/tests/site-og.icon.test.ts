import assert from 'node:assert/strict';
import test from 'node:test';

import { PNG } from 'pngjs';

import { loadSiteIcon } from '../src/application/site-og/site-og.icon.server.ts';

import { profile } from './site-og.fixture.ts';

const iconHash = 'a'.repeat(64);
const withIcon = { ...profile, iconHash };
const request = new Request('https://www.heyblog.net/og/site/38FycC0ow.png', {
  headers: { Cookie: 'session=private', 'X-Real-IP': '203.0.113.2' },
});
const loadConfig = () => ({ apiBaseUrl: 'https://api.example.test', apiWebToken: 'test-token' });
const png = PNG.sync.write(new PNG({ width: 32, height: 16 }));
const headers = { 'Content-Type': 'image/png', ETag: `"${iconHash}"` };

test('loads only cached API icon bytes and forwards no browser credentials', async () => {
  // Given / When: a cached icon is requested through the authenticated API boundary.
  const result = await loadSiteIcon(withIcon, request, {
    loadConfig,
    fetch: async (input, init) => {
      assert.equal(String(input), 'https://api.example.test/sites/id/38FycC0ow/icon');
      const sent = new Headers(init?.headers);
      assert.equal(sent.get('X-HeyBlog-Web-Token'), 'test-token');
      assert.equal(sent.get('Cookie'), null);
      assert.equal(sent.get('X-Forwarded-For'), '203.0.113.2');
      assert.equal(init?.redirect, 'error');
      assert.ok(init?.signal);
      return new Response(new Uint8Array(png), { headers });
    },
  });
  // Then: only a validated, bounded PNG becomes an inline image.
  assert.deepEqual(result, {
    kind: 'ready',
    dataUrl: `data:image/png;base64,${png.toString('base64')}`,
  });
});

test('distinguishes missing icons from transient failures and rejects stale or unsafe responses', async () => {
  // Given: each response the cached-image boundary can produce.
  const scenarios = [
    { response: new Response(null, { status: 404 }), kind: 'missing' },
    { response: new Response(null, { status: 503 }), kind: 'unavailable' },
    {
      response: new Response(new Uint8Array(png), { headers: { ...headers, ETag: '"old"' } }),
      kind: 'unavailable',
    },
    { response: new Response('<svg/>', { headers }), kind: 'unavailable' },
    { response: new Response(new Uint8Array(131073), { headers }), kind: 'unavailable' },
    {
      response: new Response(new Uint8Array(PNG.sync.write(new PNG({ width: 129, height: 1 }))), {
        headers,
      }),
      kind: 'unavailable',
    },
  ];
  for (const scenario of scenarios) {
    // When / Then: each failure selects the correct fallback/cache behavior.
    const result = await loadSiteIcon(withIcon, request, {
      loadConfig,
      fetch: async () => scenario.response,
    });
    assert.equal(result.kind, scenario.kind);
  }
  assert.deepEqual(
    await loadSiteIcon(profile, request, {
      fetch: async () => {
        throw new Error('must not fetch');
      },
    }),
    { kind: 'missing' },
  );
});

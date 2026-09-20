import assert from 'node:assert/strict';
import test from 'node:test';

import { createSiteOgHandler } from '../src/application/site-og/site-og.endpoint.server.ts';
import {
  siteOgContent,
  siteOgImagePath,
  siteProfileMetadata,
} from '../src/application/site-og/site-og.model.ts';
import { fitText, wrapText } from '../src/application/site-og/site-og.text.ts';

import { profile } from './site-og.fixture.ts';

test('uses site content and updates image versions only when visible content changes', () => {
  // Given / When: a profile, a renamed profile, and a metadata-only update.
  const metadata = siteProfileMetadata(profile);
  // Then: site-specific copy and stable, content-addressed short-ID URLs.
  assert.equal(metadata.siteName, `${profile.name} | HeyBlog`);
  assert.equal(metadata.description, profile.summary);
  assert.match(metadata.imagePath, /^\/og\/site\/38FycC0ow\.png\?v=[a-f0-9]{16}$/u);
  assert.notEqual(siteOgImagePath({ ...profile, name: '新名称' }), metadata.imagePath);
  assert.notEqual(siteOgImagePath({ ...profile, iconHash: 'a'.repeat(64) }), metadata.imagePath);
  assert.equal(siteOgContent(profile).detailUrl, 'https://www.heyblog.net/site/38FycC0ow');
  assert.equal(siteOgImagePath({ ...profile, updatedAt: '2026-09-20' }), metadata.imagePath);
  assert.deepEqual(siteOgContent(profile).classification, ['技术', '软件开发']);
  const empty = siteOgContent({ ...profile, summary: ' ', classification: null });
  assert.ok(empty.description.includes(profile.name) && empty.description.includes(profile.host));
  assert.equal(empty.classification, null);
});

test('falls back without caching when a cached site icon is temporarily unavailable', async () => {
  // Given: a site with a cached icon, whose icon service is temporarily unavailable.
  const handle = createSiteOgHandler({
    load: async () => ({ kind: 'success', data: { ...profile, iconHash: 'a'.repeat(64) } }),
    loadIcon: async () => ({ kind: 'unavailable' }),
    render: async (content) => {
      assert.equal(content.iconDataUrl, undefined);
      return new Uint8Array([137, 80, 78, 71]);
    },
  });
  // When: the site's name card is requested.
  const response = await handle(
    profile.shortId,
    new Request('https://www.heyblog.net/og/site/id.png'),
  );
  // Then: the first-letter card remains available without caching the transient fallback.
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('cache-control'), 'no-store');
});

const segments = (text: string) => [
  ...new Intl.Segmenter('zh-CN', { granularity: 'grapheme' }).segment(text),
];
const measure = (text: string) => segments(text).length * 10;

test('wraps long text with an ellipsis without splitting graphemes or exceeding line bounds', () => {
  // Given: combining marks and emoji sequences alongside CJK and Latin.
  const input = '你好e\u0301👩‍💻世界abcdef';
  // When: the last line has a hard width budget.
  const lines = wrapText(input, { width: 40, maxLines: 2, measure });
  // Then: each line fits, graphemes stay intact, and omitted content is signalled.
  assert.deepEqual(lines, ['你好e\u0301👩‍💻', '世界a…']);
  assert.ok(lines.every((line) => measure(line) <= 40));
  assert.deepEqual(wrapText('  ', { width: 40, maxLines: 2, measure }), []);
  assert.equal(fitText('1234', 40, measure), '1234');
});

test('serves PNGs and redirects UUIDs while preserving the existing ID lookup strategy', async () => {
  // Given: a real short ID returned by the API and a deterministic image renderer.
  const png = new Uint8Array([137, 80, 78, 71]);
  const handle = createSiteOgHandler({
    load: async () => ({ kind: 'success', data: profile }),
    render: async () => png,
  });
  // When: the same site is requested by short ID and UUID.
  const response = await handle(
    profile.shortId,
    new Request('https://www.heyblog.net/og/site/38FycC0ow.png'),
  );
  const redirect = await handle(
    '019ded7f-4be3-7168-8953-b64109326083',
    new Request('https://www.heyblog.net/og/site/id.png'),
  );
  // Then: only the canonical request renders; redirection targets the short-ID PNG.
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('content-type'), 'image/png');
  assert.equal(
    response.headers.get('cache-control'),
    'public, max-age=0, s-maxage=300, stale-while-revalidate=60',
  );
  assert.deepEqual(new Uint8Array(await response.arrayBuffer()), png);
  assert.equal(redirect.status, 308);
  assert.equal(redirect.headers.get('location'), siteOgImagePath(profile));
});

test('does not cache missing, unavailable, or failed renders', async () => {
  // Given: each failure exposed by the site lookup boundary.
  for (const kind of ['not-found', 'bad-request', 'unavailable'] as const) {
    const handle = createSiteOgHandler({
      load: async () => ({ kind }),
      render: async () => {
        throw new Error('must not render');
      },
    });
    // When: a social crawler requests an image.
    const response = await handle(
      'invalid',
      new Request('https://www.heyblog.net/og/site/invalid.png'),
    );
    // Then: status semantics and cache rules match the site page.
    assert.equal(response.status, kind === 'unavailable' ? 503 : 404);
    assert.equal(response.headers.get('cache-control'), 'no-store');
    if (kind === 'unavailable') assert.equal(response.headers.get('retry-after'), '30');
  }
  const handle = createSiteOgHandler({
    load: async () => ({ kind: 'success', data: profile }),
    render: async () => {
      throw new Error('internal font path');
    },
  });
  const response = await handle(
    profile.shortId,
    new Request('https://www.heyblog.net/og/site/id.png'),
  );
  assert.equal(response.status, 503);
  assert.equal(response.headers.get('cache-control'), 'no-store');
  assert.equal(await response.text(), '');
});

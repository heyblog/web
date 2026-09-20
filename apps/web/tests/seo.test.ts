import assert from 'node:assert/strict';
import test from 'node:test';

import { resolvePageMetadata } from '../src/shared/seo.ts';

test('appends the brand once for both raw and already branded page titles', () => {
  // Given / When: raw and already branded titles reach the shared layout.
  const raw = resolvePageMetadata({ pathname: '/site/123456789', title: '测试站点' });
  const branded = resolvePageMetadata({ pathname: '/login', title: '登录 | HeyBlog' });
  // Then: the suffix occurs exactly once.
  assert.equal(raw.title, '测试站点 | HeyBlog');
  assert.equal(branded.title, '登录 | HeyBlog');
});

test('resolves site-specific image metadata without changing platform defaults', () => {
  // Given / When: one page supplies a PNG and site name, another uses defaults.
  const site = resolvePageMetadata({
    pathname: '/site/123456789',
    title: '测试站点',
    siteName: '测试站点 | HeyBlog',
    description: '站点简介',
    imagePath: '/og/site/123456789.png?v=abc',
    imageType: 'image/png',
    imageWidth: 1200,
    imageHeight: 630,
  });
  const platform = resolvePageMetadata({ pathname: '/' });
  // Then: all overrides are preserved and the default SVG stays correctly typed.
  assert.equal(site.siteName, '测试站点 | HeyBlog');
  assert.equal(site.description, '站点简介');
  assert.equal(site.imageUrl, 'https://www.heyblog.net/og/site/123456789.png?v=abc');
  assert.equal(site.imageType, 'image/png');
  assert.equal(site.imageWidth, 1200);
  assert.equal(site.imageHeight, 630);
  assert.equal(platform.siteName, 'HeyBlog');
  assert.equal(platform.imageType, 'image/svg+xml');
});

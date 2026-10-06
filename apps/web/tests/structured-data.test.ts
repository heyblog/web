import assert from 'node:assert/strict';
import test from 'node:test';

import { contentDescription } from '../src/shared/content/description.ts';
import { resolvePageMetadata } from '../src/shared/seo.ts';
import {
  articleStructuredData,
  breadcrumbStructuredData,
  serializeStructuredData,
} from '../src/shared/structured-data.ts';

test('script-safe structured data preserves text and author attribution', () => {
  const title = '</script><script>alert(1)</script>&\u2028';
  const article = articleStructuredData({
    title,
    description: '简介',
    path: '/blog/example',
    authors: [{ label: '作者', href: '/members#author' }],
    publishedTime: '2026-10-02',
    modifiedTime: 'invalid',
  });
  const serialized = serializeStructuredData([article]);
  assert.doesNotMatch(serialized, /<|>|&|\u2028/);
  assert.ok(serialized.includes('\\u003c'));
  const parsed: unknown = JSON.parse(serialized);
  const { dateModified, ...expected } = article;
  assert.equal(dateModified, undefined);
  assert.deepEqual(parsed, { '@context': 'https://schema.org', '@graph': [expected] });
  assert.equal(article.datePublished, '2026-10-02T00:00:00.000Z');
  assert.deepEqual(article.author, [
    { '@type': 'Person', name: '作者', url: 'https://www.heyblog.net/members#author' },
  ]);
  assert.equal(breadcrumbStructuredData([{ name: '首页', path: '/' }])['@type'], 'BreadcrumbList');
});

test('metadata indexes normal pagination but suppresses explicit filters, private pages and errors', () => {
  for (const path of ['/site', '/announcements', '/site/', '/announcements/']) {
    assert.equal(resolvePageMetadata({ pathname: path }).robots, 'index, follow');
    const paged = resolvePageMetadata({ pathname: path, canonicalPath: path, search: '?page=2' });
    assert.equal(paged.canonicalUrl, `https://www.heyblog.net${path.replace(/\/$/, '')}?page=2`);
    assert.equal(paged.robots, 'index, follow');
    assert.equal(
      resolvePageMetadata({ pathname: path, search: '?page=1' }).canonicalUrl,
      `https://www.heyblog.net${path.replace(/\/$/, '')}`,
    );
    for (const search of [
      '?page=0',
      '?page=2&page=3',
      '?q=技术',
      '?sort=random',
      '?seed=shuffle',
    ]) {
      assert.match(resolvePageMetadata({ pathname: path, search }).robots, /noindex/);
    }
  }
  for (const pathname of ['/management/tags', '/login', '/site/submissions/new', '/site/go'])
    assert.match(resolvePageMetadata({ pathname }).robots, /noindex/);
  assert.match(resolvePageMetadata({ pathname: '/site/123456789', status: 503 }).robots, /noindex/);
  assert.equal(
    resolvePageMetadata({ pathname: '/blog/example/' }).canonicalUrl,
    'https://www.heyblog.net/blog/example',
  );
});

test('descriptions use plain paragraph text, skip code and HTML, and bound Unicode characters', () => {
  assert.equal(
    contentDescription(
      '# 标题\n\n```js\nsecret\n```\n\n首段 **重点** 和 [链接](https://example.test)。',
      'fallback',
    ),
    '首段 重点 和 链接。',
  );
  assert.equal(contentDescription('<script>alert(1)</script>', 'fallback'), 'fallback');
  assert.equal(contentDescription(undefined, '简介'), '简介');
  assert.equal(Array.from(contentDescription('🙂'.repeat(200), 'fallback')).length, 160);
});

import assert from 'node:assert/strict';
import test from 'node:test';

import { loadSitemapItems, SitemapUnavailableError } from '../src/api/sitemap/sitemap.server.ts';
import { createSitemapService } from '../src/application/seo/sitemap-cache.server.ts';
import { createSitemapDocuments } from '../src/shared/sitemap.ts';

const request = (path = '/sitemap.xml', headers?: HeadersInit) =>
  new Request(`https://www.heyblog.net${path}`, { headers });
const uuid = (number: number) => `00000000-0000-7000-8000-${number.toString(16).padStart(12, '0')}`;
const site = (number: number) => ({
  id: uuid(number),
  shortId: number.toString().padStart(9, '0'),
});
const config = () => ({
  apiBaseUrl: 'http://api.test',
  apiWebToken: 'test-service-token-0123456789abcdef',
});

test('complete XML splits large collections, keeps canonical URLs, escapes XML and omits unknown lastmod', () => {
  const documents = createSitemapDocuments({
    static: [{ path: '/blog/中文&笔记', lastmod: '2026-10-02T00:00:00Z' }],
    sites: Array.from({ length: 10_001 }, (_, number) => ({
      path: `/site/${site(number).shortId}`,
    })),
    announcements: [],
  });
  assert.ok(documents.get('/sitemap.xml')?.includes('https://www.heyblog.net/sitemap-sites-2.xml'));
  assert.equal(documents.get('/sitemap-sites-1.xml')?.match(/<url>/g)?.length, 10_000);
  assert.equal(documents.get('/sitemap-sites-2.xml')?.match(/<url>/g)?.length, 1);
  assert.match(
    documents.get('/sitemap-static-1.xml') ?? '',
    /%E4%B8%AD%E6%96%87&amp;%E7%AC%94%E8%AE%B0/,
  );
  assert.doesNotMatch(documents.get('/sitemap-sites-1.xml') ?? '', /lastmod/);
  assert.match(documents.get('/sitemap-announcements-1.xml') ?? '', /<urlset[^>]*>\n<\/urlset>/);
  for (const path of [
    '/management/tags',
    '/login',
    'https://external.test/entry',
    '/site?page=2',
  ]) {
    assert.throws(() =>
      createSitemapDocuments({ static: [{ path }], sites: [], announcements: [] }),
    );
  }
});

test('cursor reads validate IDs, forward only the service token, and reject malformed pages', async () => {
  const first = Array.from({ length: 1_000 }, (_, number) => site(number + 1));
  const paths: string[] = [];
  const result = await loadSitemapItems('sites', AbortSignal.timeout(5_000), {
    loadConfig: config,
    fetch: async (input, init) => {
      const path = String(input);
      paths.push(path);
      const headers = new Headers(init?.headers);
      assert.equal(headers.get('Cookie'), null);
      assert.equal(headers.get('X-HeyBlog-Web-Token'), config().apiWebToken);
      return Response.json(
        paths.length === 1
          ? { items: first, nextAfter: uuid(1_000) }
          : { items: [site(1_001)], nextAfter: null },
      );
    },
  });
  assert.equal(result.length, 1_001);
  assert.ok(paths[1]?.includes(`after=${uuid(1_000)}`));
  for (const value of [
    { items: [site(1), site(1)], nextAfter: null },
    { items: [], nextAfter: uuid(1) },
    { items: [{ id: uuid(1), shortId: '../private' }], nextAfter: null },
  ]) {
    await assert.rejects(
      loadSitemapItems('sites', AbortSignal.timeout(5_000), {
        loadConfig: config,
        fetch: async () => Response.json(value),
      }),
      SitemapUnavailableError,
    );
  }
});

test('public snapshot coalesces callers, has a fixed expiry, and revalidates ETags without partial fallback', async () => {
  let clock = 0;
  let reads = 0;
  let failed = false;
  const serve = createSitemapService({
    now: () => clock,
    loadStatic: async () => [{ path: '/' }],
    loadDynamic: async (kind) => {
      reads++;
      if (failed) throw new SitemapUnavailableError();
      return kind === 'sites'
        ? [site(1)]
        : [
            {
              id: uuid(2),
              startsAt: '2026-10-02T00:00:00Z',
              publishedAt: '2026-10-01T00:00:00Z',
              updatedAt: '2026-10-01T01:00:00Z',
            },
          ];
    },
  });
  const [index, sites] = await Promise.all([
    serve('/sitemap.xml', request()),
    serve('/sitemap-sites-1.xml', request('/sitemap-sites-1.xml')),
  ]);
  assert.equal(reads, 2);
  assert.equal(index.status, 200);
  assert.equal(sites.status, 200);
  clock = 59_000;
  const cached = await serve(
    '/sitemap.xml',
    request('/sitemap.xml', { 'If-None-Match': index.headers.get('ETag') ?? '' }),
  );
  assert.equal(cached.status, 304);
  assert.equal(await cached.text(), '');
  assert.match(cached.headers.get('Cache-Control') ?? '', /s-maxage=1,/);
  const announcement = await serve('/sitemap-announcements-1.xml', request());
  assert.match(await announcement.text(), /<lastmod>2026-10-02T00:00:00.000Z<\/lastmod>/);
  assert.equal((await serve('/sitemap-sites-2.xml', request())).status, 404);
  clock = 60_000;
  failed = true;
  const unavailable = await serve('/sitemap.xml', request());
  assert.equal(unavailable.status, 503);
  assert.equal(unavailable.headers.get('Cache-Control'), 'no-store');
  assert.equal(unavailable.headers.get('Retry-After'), '30');
  assert.doesNotMatch(await unavailable.text(), /<sitemapindex/);
  failed = false;
  assert.equal((await serve('/sitemap.xml', request())).status, 200);
  const head = await serve(
    '/sitemap.xml',
    new Request('https://www.heyblog.net/sitemap.xml', { method: 'HEAD' }),
  );
  assert.equal(head.status, 200);
  assert.equal(await head.text(), '');
});

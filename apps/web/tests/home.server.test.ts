import assert from 'node:assert/strict';
import test from 'node:test';

import { refreshHome } from '../src/application/home/home.browser.ts';
import { createHomeMockState, parseHomeMockMode } from '../src/application/home/home.mock.ts';
import {
  formatSiteJoinedAt,
  formatSiteUpdatedAt,
  siteDetailPath,
} from '../src/application/home/home.shared.ts';
import {
  createBlogCardTags,
  resolveAnchoredDialogLayout,
  resolveVisibleTagCount,
} from '../src/components/home/blog-card-layout.shared.ts';

test('formats joined dates without depending on the server timezone', () => {
  assert.equal(formatSiteJoinedAt('2025-01-02T23:30:00-08:00'), '2025年1月加入');
  assert.equal(formatSiteJoinedAt('invalid'), '已加入目录');
});

test('formats site information update dates independently from joined dates', () => {
  assert.equal(formatSiteUpdatedAt('2025-02-03T23:30:00-08:00'), '2025年2月4日更新');
  assert.equal(formatSiteUpdatedAt('invalid'), '信息更新时间未知');
});

test('always uses the short ID for internal site detail routes', () => {
  assert.equal(siteDetailPath({ shortId: 'A1b2C3d4E' }), '/site/A1b2C3d4E');
});

test('enables supported home mock modes only when development mocks are allowed', () => {
  assert.equal(parseHomeMockMode('cards', true), 'cards');
  assert.equal(parseHomeMockMode('empty', true), 'empty');
  assert.equal(parseHomeMockMode('error', true), 'error');
  assert.equal(parseHomeMockMode('unknown', true), null);
  assert.equal(parseHomeMockMode('cards', false), null);
});

test('provides more than six mock cards with distinct optional field states', () => {
  const state = createHomeMockState('cards');
  const sites = state.home?.sites ?? [];

  assert.equal(state.unavailable, false);
  assert.equal(sites.length, 12);
  assert.deepEqual(
    new Set(sites.map((site) => site.accessScope)),
    new Set(['ALL', 'CN_ONLY', 'GLOBAL_ONLY']),
  );
  assert.deepEqual(
    new Set(sites.flatMap((site) => (site.defaultFeed ? [site.defaultFeed.format] : []))),
    new Set(['RSS', 'ATOM', 'JSON']),
  );
  assert.ok(sites.some((site) => site.summary === ''));
  assert.ok(sites.some((site) => site.topics.length === 0));
  assert.ok(sites.some((site) => site.warnings.length > 0));
  assert.ok(sites.some((site) => site.defaultFeed === null && site.sitemapUrl !== null));
});

test('provides explicit empty and unavailable home mock states', () => {
  const empty = createHomeMockState('empty');
  const error = createHomeMockState('error');

  assert.equal(empty.unavailable, false);
  assert.deepEqual(empty.home?.sites, []);
  assert.equal(error.unavailable, true);
  assert.equal(error.home, null);
});

test('refreshes home data through the same-origin web endpoint', async () => {
  let requestURL: string | URL | Request | undefined;
  let requestInit: RequestInit | undefined;
  const expected = createHomeMockState('cards').home;
  assert.ok(expected);

  const result = await refreshHome(undefined, async (input, init) => {
    requestURL = input;
    requestInit = init;
    return Response.json(expected);
  });

  assert.equal(requestURL, '/api/home');
  assert.equal(requestInit?.cache, 'no-store');
  assert.equal(new Headers(requestInit?.headers).get('Accept'), 'application/json');
  assert.deepEqual(result, expected);
});

test('rejects failed home refresh responses without exposing their body', async () => {
  await assert.rejects(
    refreshHome(undefined, async () => new Response('internal details', { status: 503 })),
    /status 503/,
  );
});

test('orders blog card tags by warning, primary, and secondary roles', () => {
  const tags = createBlogCardTags({
    warnings: [{ name: '访问较慢', slug: 'slow-access', description: '部分网络访问较慢。' }],
    topics: [
      { name: '写作', slug: 'writing', role: 'SECONDARY' },
      { name: '技术', slug: 'technology', role: 'PRIMARY' },
      { name: '开源', slug: 'open-source', role: 'SECONDARY' },
    ],
  });

  assert.deepEqual(
    tags.map((tag) => [tag.label, tag.tone]),
    [
      ['访问较慢', 'warning'],
      ['技术', 'primary'],
      ['写作', 'secondary'],
      ['开源', 'secondary'],
    ],
  );
});

test('uses the uncategorized tag after warnings when a primary topic is absent', () => {
  const tags = createBlogCardTags({
    warnings: [{ name: '仅英文', slug: 'english-only', description: '' }],
    topics: [{ name: '随笔', slug: 'essay', role: 'SECONDARY' }],
  });

  assert.deepEqual(
    tags.map((tag) => tag.label),
    ['仅英文', '未分类', '随笔'],
  );
});

test('keeps all blog card tags that fit within two rows', () => {
  assert.equal(
    resolveVisibleTagCount({
      containerWidth: 100,
      tagWidths: [40, 40, 40],
      counterWidths: [0, 24, 24, 24],
      gap: 6,
    }),
    3,
  );
});

test('reserves room for a hidden-tag counter on the compact single row', () => {
  assert.equal(
    resolveVisibleTagCount({
      containerWidth: 100,
      tagWidths: [40, 40, 40],
      counterWidths: [0, 24, 24, 24],
      gap: 6,
      maxRows: 1,
    }),
    1,
  );
});

test('reserves room for the hidden-tag counter in the second row', () => {
  assert.equal(
    resolveVisibleTagCount({
      containerWidth: 100,
      tagWidths: [40, 40, 40, 40, 40],
      counterWidths: [0, 24, 24, 24, 24, 24],
      gap: 6,
    }),
    3,
  );
});

test('removes an additional visible tag when a wider counter requires it', () => {
  assert.equal(
    resolveVisibleTagCount({
      containerWidth: 100,
      tagWidths: [45, 45, 45, 45, 45],
      counterWidths: [0, 55, 55, 55, 55, 55],
      gap: 6,
    }),
    2,
  );
});

test('defers blog card tag clipping until a measurable width is available', () => {
  assert.equal(
    resolveVisibleTagCount({
      containerWidth: 0,
      tagWidths: [40, 40, 40],
      counterWidths: [0, 24, 24, 24],
      gap: 6,
    }),
    3,
  );
});

test('anchors an expanded card inside the viewport without changing its source width', () => {
  const layout = resolveAnchoredDialogLayout({
    source: { left: 24, top: 120, width: 320, height: 256 },
    dialogHeight: 480,
    viewport: { width: 375, height: 667 },
  });

  assert.deepEqual(layout, {
    left: 24,
    top: 120,
    width: 320,
    maxHeight: 635,
  });
});

test('clamps an expanded card to the viewport inset when its source is near an edge', () => {
  const layout = resolveAnchoredDialogLayout({
    source: { left: 900, top: 650, width: 420, height: 256 },
    dialogHeight: 620,
    viewport: { width: 1280, height: 720 },
  });

  assert.deepEqual(layout, {
    left: 844,
    top: 84,
    width: 420,
    maxHeight: 688,
  });
});

import assert from 'node:assert/strict';
import test from 'node:test';

import {
  activeNavigationHref,
  resolveBrandVisibility,
  resolvePublicAccountEntry,
  sortNavigation,
} from '../src/shared/public-header.shared.ts';

test('maps public account navigation from session presence', () => {
  assert.deepEqual(resolvePublicAccountEntry(false), { href: '/login', label: '登录' });
  assert.deepEqual(resolvePublicAccountEntry(true), { href: '/dashboard', label: '账号' });
});

test('sorts navigation without mutating the configured order', () => {
  const items = [
    { label: '成员', href: '/members', match: 'prefix', sort: 30 },
    { label: '博客列表', href: '/site', match: 'prefix', sort: 10 },
  ] as const;
  assert.deepEqual(
    sortNavigation(items).map((item) => item.href),
    ['/site', '/members'],
  );
  assert.equal(items[0].href, '/members');
});

test('selects the most specific destination and does not match path fragments', () => {
  const items = [
    { label: '博客列表', href: '/site', match: 'prefix', sort: 10 },
    { label: '提交新站点', href: '/site/submissions/new', match: 'exact', sort: 20 },
  ] as const;
  assert.equal(activeNavigationHref(items, '/site/submissions/new/'), '/site/submissions/new');
  assert.equal(activeNavigationHref(items, '/site/123456789'), '/site');
  assert.equal(activeNavigationHref(items, '/sitemap'), undefined);
  assert.equal(activeNavigationHref(items, '/'), undefined);
});
test('keeps the brand visible outside the home page', () => {
  assert.equal(resolveBrandVisibility({ home: false, heroBottom: null, threshold: 76 }), true);
});

test('reveals the home brand only after the hero crosses the header threshold', () => {
  assert.equal(resolveBrandVisibility({ home: true, heroBottom: 300, threshold: 76 }), false);
  assert.equal(resolveBrandVisibility({ home: true, heroBottom: 76, threshold: 76 }), true);
  assert.equal(resolveBrandVisibility({ home: true, heroBottom: null, threshold: 76 }), false);
});

test('matches random menu destinations by pathname while retaining preview query', () => {
  const items = [
    { label: '博客列表', href: '/site', match: 'prefix', sort: 10 },
    { label: '随机前往', href: '/site/go?preview=true', match: 'exact', sort: 15 },
  ] as const;
  for (const path of ['/site/go', '/site/go/', '/site/go?level1=技术']) {
    assert.equal(activeNavigationHref(items, path), '/site/go?preview=true');
  }
});

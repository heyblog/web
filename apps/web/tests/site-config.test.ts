import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import { siteConfig } from '../src/site.config.ts';

test('aligns the management sidebar CSS with its 80rem desktop controller', async () => {
  const source = await readFile(
    new URL('../src/layouts/ManagementLayout.astro', import.meta.url),
    'utf8',
  );

  assert.doesNotMatch(source, /\bxl:/u);
  assert.match(source, /lg:grid-cols-\[17rem_minmax\(0,1fr\)\]/u);
});

test('keeps documents available without exposing them in the public navigation', () => {
  assert.deepEqual(
    siteConfig.navigation.primary.map((item) => item.label),
    ['博客列表', '项目动态', '成员'],
  );
  assert.equal(
    siteConfig.navigation.primary.some((item) => item.href === '/docs' || item.href === '/'),
    false,
  );
});

test('separates direct blog submission from collection actions', () => {
  assert.deepEqual(siteConfig.navigation.submission.primary, {
    label: '提交博客',
    href: '/site/submissions/new',
    match: 'exact',
    sort: 10,
  });
  assert.deepEqual(
    siteConfig.navigation.submission.menu.map(({ label, href }) => ({ label, href })),
    [
      { label: '更新收录信息', href: '/site/submissions/update' },
      { label: '移除收录', href: '/site/submissions/delete' },
      { label: '恢复收录', href: '/site/submissions/restore' },
      { label: '查询提交进度', href: '/site/submissions/query' },
    ],
  );
  assert.equal('hub' in siteConfig.navigation.submission, false);
});

test('permanently redirects the retired submission hub', async () => {
  const source = await readFile(
    new URL('../src/pages/site/submissions/index.astro', import.meta.url),
    'utf8',
  );

  assert.match(source, /return Astro\.redirect\('\/site\/submissions\/new', 308\);/u);
});

test('trusts the canonical production origin behind the reverse proxy', async () => {
  const source = await readFile(new URL('../astro.config.ts', import.meta.url), 'utf8');

  assert.match(source, /allowedDomains:/u);
  assert.match(source, /hostname: 'www\.heyblog\.net'/u);
});

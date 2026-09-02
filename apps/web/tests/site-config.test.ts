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
    siteConfig.navigation.map((item) => item.label),
    ['首页', '博客列表', '项目动态', '成员', '提交博客'],
  );
  assert.equal(
    siteConfig.navigation.some((item) => item.href === '/docs'),
    false,
  );
  const submissions = siteConfig.navigation.find((item) => item.href === '/site/submissions');
  assert.deepEqual(
    submissions?.children?.map((item) => item.href),
    [
      '/site/submissions/new',
      '/site/submissions/update',
      '/site/submissions/delete',
      '/site/submissions/restore',
      '/site/submissions/query',
    ],
  );
});

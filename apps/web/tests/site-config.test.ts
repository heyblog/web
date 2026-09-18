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
  assert.deepEqual(
    siteConfig.navigation.submission.map((item) => item.href),
    [
      '/site/submissions',
      '/site/submissions/new',
      '/site/submissions/query',
      '/site/submissions/update',
      '/site/submissions/delete',
      '/site/submissions/restore',
    ],
  );
});

test('trusts the canonical production origin behind the reverse proxy', async () => {
  const source = await readFile(new URL('../astro.config.ts', import.meta.url), 'utf8');

  assert.match(source, /allowedDomains:/u);
  assert.match(source, /hostname: 'www\.heyblog\.net'/u);
});

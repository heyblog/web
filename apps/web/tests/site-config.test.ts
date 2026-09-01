import assert from 'node:assert/strict';
import test from 'node:test';

import { siteConfig } from '../src/site.config.ts';

test('keeps documents available without exposing them in the public navigation', () => {
  assert.deepEqual(
    siteConfig.navigation.map((item) => item.label),
    ['首页', '博客', '成员'],
  );
  assert.equal(
    siteConfig.navigation.some((item) => item.href === '/docs'),
    false,
  );
});

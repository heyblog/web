import assert from 'node:assert/strict';
import test from 'node:test';

import {
  resolveBrandVisibility,
  resolvePublicAccountEntry,
} from '../src/shared/public-header.shared.ts';

test('maps public account navigation from session presence', () => {
  assert.deepEqual(resolvePublicAccountEntry(false), { href: '/login', label: '登录' });
  assert.deepEqual(resolvePublicAccountEntry(true), { href: '/dashboard', label: '账号' });
});

test('keeps the brand visible outside the home page', () => {
  assert.equal(resolveBrandVisibility({ home: false, heroBottom: null, threshold: 76 }), true);
});

test('reveals the home brand only after the hero crosses the header threshold', () => {
  assert.equal(resolveBrandVisibility({ home: true, heroBottom: 300, threshold: 76 }), false);
  assert.equal(resolveBrandVisibility({ home: true, heroBottom: 76, threshold: 76 }), true);
  assert.equal(resolveBrandVisibility({ home: true, heroBottom: null, threshold: 76 }), false);
});

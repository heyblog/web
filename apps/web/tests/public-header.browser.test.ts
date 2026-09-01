import assert from 'node:assert/strict';
import test from 'node:test';

import { resolveBrandVisibility } from '../src/shared/public-header.shared.ts';

test('keeps the brand visible outside the home page', () => {
  assert.equal(resolveBrandVisibility({ home: false, heroBottom: null, threshold: 76 }), true);
});

test('reveals the home brand only after the hero crosses the header threshold', () => {
  assert.equal(resolveBrandVisibility({ home: true, heroBottom: 300, threshold: 76 }), false);
  assert.equal(resolveBrandVisibility({ home: true, heroBottom: 76, threshold: 76 }), true);
  assert.equal(resolveBrandVisibility({ home: true, heroBottom: null, threshold: 76 }), false);
});

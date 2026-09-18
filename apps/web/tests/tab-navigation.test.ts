import assert from 'node:assert/strict';
import test from 'node:test';

import { nextTabIndex } from '../src/shared/tab-navigation.ts';

test('tabs wrap in both directions and support first and last shortcuts', () => {
  assert.equal(nextTabIndex('ArrowRight', 3, 4), 0);
  assert.equal(nextTabIndex('ArrowLeft', 0, 4), 3);
  assert.equal(nextTabIndex('ArrowRight', 1, 4), 2);
  assert.equal(nextTabIndex('Home', 2, 4), 0);
  assert.equal(nextTabIndex('End', 1, 4), 3);
});

test('tabs leave native keyboard keys alone and handle an empty set', () => {
  assert.equal(nextTabIndex('Tab', 1, 4), undefined);
  assert.equal(nextTabIndex('Enter', 1, 4), undefined);
  assert.equal(nextTabIndex('ArrowLeft', 0, 0), undefined);
});

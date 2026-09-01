import assert from 'node:assert/strict';
import test from 'node:test';

import {
  resolveContentScrollProgress,
  updateContentProgress,
} from '../src/components/content-page/content-toc-progress.browser.ts';

test('maps the page scroll range to exact progress endpoints', () => {
  const scrollHeight = 3863;
  const viewportHeight = 900;

  assert.equal(resolveContentScrollProgress(0, scrollHeight, viewportHeight), 0);
  assert.equal(resolveContentScrollProgress(1481.5, scrollHeight, viewportHeight), 0.5);
  assert.equal(resolveContentScrollProgress(2963, scrollHeight, viewportHeight), 1);
  assert.equal(resolveContentScrollProgress(4000, scrollHeight, viewportHeight), 1);
  assert.equal(resolveContentScrollProgress(-20, scrollHeight, viewportHeight), 0);
  assert.equal(resolveContentScrollProgress(0, 800, viewportHeight), 0);
});

test("overrides Tailwind's zero scale without writing a competing transform", () => {
  const cases = [
    { progress: 0, percent: '0', scale: '0 1' },
    { progress: 0.607398, percent: '61', scale: '0.607398 1' },
    { progress: 1, percent: '100', scale: '1 1' },
  ] as const;

  for (const scenario of cases) {
    const attributes = new Map<string, string>();
    const style = { scale: '0 1', transform: 'none' };

    updateContentProgress(
      { setAttribute: (name, value) => attributes.set(name, value) },
      style,
      scenario.progress,
    );

    assert.equal(attributes.get('aria-valuenow'), scenario.percent);
    assert.equal(style.scale, scenario.scale);
    assert.equal(style.transform, 'none');
  }
});

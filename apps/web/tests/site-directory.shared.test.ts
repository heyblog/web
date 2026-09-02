import assert from 'node:assert/strict';
import test from 'node:test';

import {
  buildSiteDirectorySearchParams,
  createDailyDirectorySeed,
  parseSiteDirectorySearchParams,
} from '../src/application/site-directory/site-directory.shared.ts';

test('creates the daily directory seed from the China calendar date', () => {
  const date = new Date('2026-09-01T16:30:00.000Z');

  assert.equal(createDailyDirectorySeed(date), 'site-directory:2026-09-02');
});

test('round trips repeated directory filters and stable random state', () => {
  const query = parseSiteDirectorySearchParams(
    new URLSearchParams(
      'page=3&q=astro&primary=technology&primary=life&secondary=design&warning=slow-access&technology=astro&access=ALL&feed=with&status=abnormal&sort=random&order=desc&seed=site-directory%3Ashared',
    ),
  );

  assert.equal(
    buildSiteDirectorySearchParams(query).toString(),
    'page=3&q=astro&feed=with&status=abnormal&sort=random&order=desc&seed=site-directory%3Ashared&primary=technology&primary=life&secondary=design&warning=slow-access&technology=astro&access=ALL',
  );
});

test('uses safe defaults for unsupported directory query values', () => {
  const query = parseSiteDirectorySearchParams(
    new URLSearchParams(
      'page=0&feed=invalid&status=removed&sort=visits&order=sideways&access=LOCAL',
    ),
  );

  assert.equal(query.page, 1);
  assert.equal(query.feed, 'any');
  assert.equal(query.status, 'normal');
  assert.equal(query.sort, 'random');
  assert.equal(query.order, 'desc');
  assert.deepEqual(query.access, []);
});

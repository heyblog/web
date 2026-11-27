import assert from 'node:assert/strict';
import test from 'node:test';

import { claimListQuery } from '../src/application/site-management/claims-query.ts';
import { listPageNumber } from '../src/application/site-management/pagination.ts';
import {
  reviewListPath,
  reviewListQuery,
} from '../src/application/site-submission/review-list-query.ts';

test('review pagination preserves explicit empty filters across forward and back navigation', () => {
  for (const status of ['', 'PENDING', 'APPROVED', 'REJECTED']) {
    for (const action of ['', 'CREATE', 'UPDATE', 'DELETE', 'RESTORE']) {
      const filters = { status, action };
      for (const page of [2, 3, 1]) {
        const destination = new URL(reviewListPath(filters, page), 'https://web.example.test');
        assert.equal(destination.pathname, '/management/site-submissions');
        assert.equal(destination.searchParams.get('status'), status);
        assert.equal(destination.searchParams.get('action'), action);
        const resolved = reviewListQuery(destination.searchParams);
        assert.equal(resolved.status, status);
        assert.equal(resolved.action, action);
        assert.equal(resolved.page, page);
        assert.equal(resolved.query.get('status'), status || null);
        assert.equal(resolved.query.get('action'), action || null);
      }
    }
  }
});

test('review entry defaults to pending only when status is absent', () => {
  assert.equal(reviewListQuery(new URLSearchParams()).status, 'PENDING');
  assert.equal(reviewListQuery(new URLSearchParams('status=')).status, '');
});

test('list pages use the API page range', () => {
  for (const [value, expected] of [
    ['', 1],
    ['invalid', 1],
    ['-3', 1],
    ['1001', 1001],
    ['1000000', 1_000_000],
    ['1000001', 1_000_000],
  ] as const) {
    assert.equal(listPageNumber(new URLSearchParams({ page: value })), expected);
  }
});

test('claim pagination reaches page 1001 for account and management lists', () => {
  for (const management of [false, true]) {
    const result = claimListQuery(new URLSearchParams('page=1001&status=&method='), {
      management,
    });
    assert.equal(result.page, 1001);
    assert.equal(result.query.get('page'), '1001');
  }
});

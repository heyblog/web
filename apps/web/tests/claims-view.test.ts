import assert from 'node:assert/strict';
import test from 'node:test';

import { claimListQuery } from '../src/application/site-management/claims-query.ts';
import {
  claimViewHref,
  claimWorkspaceView,
} from '../src/application/site-management/claims-view.ts';

test('claim entry links and list navigation select the relevant workspace panel', () => {
  for (const [search, expected] of [
    ['', 'create'],
    ['short_id=abc123456&page=2', 'create'],
    ['purpose=address&status=PENDING', 'create'],
    ['status=VERIFIED', 'records'],
    ['method=', 'records'],
    ['page=2', 'records'],
    ['view=records&short_id=abc123456', 'records'],
    ['view=create&status=VERIFIED', 'create'],
    ['view=invalid', 'create'],
    ['view=invalid&page=2', 'records'],
  ] as const) {
    const view = claimWorkspaceView(new URLSearchParams(search));
    assert.equal(view, expected, search);
  }
});

test('panel links preserve site context and filters without leaking presentation state to the API', () => {
  const search = new URLSearchParams('short_id=abc123456&purpose=address&status=PENDING&page=2');
  const href = claimViewHref(search, 'records');
  const url = new URL(href, 'https://heyblog.test');
  assert.equal(url.pathname, '/dashboard/claims');
  assert.equal(url.searchParams.get('short_id'), 'abc123456');
  assert.equal(url.searchParams.get('purpose'), 'address');
  assert.equal(url.searchParams.get('status'), 'PENDING');
  assert.equal(url.searchParams.get('page'), '2');
  assert.equal(claimWorkspaceView(url.searchParams), 'records');
  assert.equal(search.has('view'), false);
  assert.equal(
    claimListQuery(url.searchParams).query.toString(),
    'page=2&page_size=20&status=PENDING',
  );
});

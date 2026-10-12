import assert from 'node:assert/strict';
import test from 'node:test';

import { ClaimProblem, createClaimsTransport } from '../src/api/site-management/claims.browser.ts';
import {
  parseClaim,
  parseClaimContext,
  parseClaimPage,
  parseOwnership,
} from '../src/api/site-management/claims.types.ts';
import {
  claimDestination,
  claimLabel,
  claimMessage,
  verifiedClaimAddress,
} from '../src/application/site-management/claims.model.ts';
import { claimListQuery } from '../src/application/site-management/claims-query.ts';

import { claimFixture, contextFixture, ownershipFixture } from './site-claims.fixture.ts';

test('claim DTOs validate capabilities, expiration and paging before reaching the page', () => {
  assert.equal(parseClaim(claimFixture).site_name, 'Example');
  assert.throws(() => parseClaim({ ...claimFixture, can_manage: 'true' }));
  assert.throws(() => parseClaim({ ...claimFixture, expires_at: 'tomorrow' }));
  assert.throws(() => parseClaimPage({ items: [], has_more: 'true' }));
  assert.equal(parseClaimPage({ items: [claimFixture], has_more: true }).has_more, true);
  assert.deepEqual(parseClaimContext(contextFixture), contextFixture);
  assert.deepEqual(parseOwnership(ownershipFixture), ownershipFixture);
  assert.throws(() =>
    parseOwnership({ ...ownershipFixture, current: { ...ownershipFixture.current, revision: 0 } }),
  );
});
test('only current management permission and a verified unused address proof enable editor prefill', () => {
  const proof = {
    ...claimFixture,
    status: 'VERIFIED',
    can_manage: true,
    address: 'https://new.example.test/',
  } as const;
  assert.equal(verifiedClaimAddress(proof, proof.short_id), proof.address);
  assert.equal(
    claimDestination(proof),
    `/dashboard/sites/${proof.short_id}?verified_claim=${proof.id}#information`,
  );
  for (const invalid of [
    { ...proof, can_manage: false },
    { ...proof, consumed_at: proof.created_at },
    { ...proof, status: 'PENDING' as const },
    { ...proof, address: 'https://user:password@example.test/' },
    { ...proof, address: 'ftp://example.test/' },
  ])
    assert.equal(verifiedClaimAddress(invalid, proof.short_id), undefined);
  assert.equal(verifiedClaimAddress(proof, 'other1234'), undefined);
  assert.equal(claimDestination({ ...proof, can_manage: false }), null);
});
test('claim filters are bounded and manual/expired states have distinct recovery labels', () => {
  assert.equal(claimLabel({ ...claimFixture, method: 'MANUAL', expires_at: undefined }), '待审核');
  assert.equal(claimLabel({ ...claimFixture, expires_at: '2020-01-01T00:00:00Z' }), '已过期');
  const { page, query } = claimListQuery(
    new URLSearchParams('page=2&status=PENDING&method=MANUAL&extra=private'),
  );
  assert.equal(page, 2);
  assert.equal(query.toString(), 'page=2&page_size=20&status=PENDING&method=MANUAL');
  assert.equal(
    claimListQuery(new URLSearchParams('page=-1&status=INVALID')).query.toString(),
    'page=1&page_size=20',
  );
  assert.equal(
    claimListQuery(new URLSearchParams(), { management: true }).query.toString(),
    'page=1&page_size=20&status=PENDING&method=MANUAL',
  );
  assert.equal(
    claimListQuery(new URLSearchParams('status=&method='), { management: true }).query.toString(),
    'page=1&page_size=20',
  );
  assert.equal(
    claimListQuery(new URLSearchParams('page=2&status=VERIFIED'), {
      management: true,
    }).query.toString(),
    'page=2&page_size=20&status=VERIFIED&method=MANUAL',
  );
});
test('verification errors preserve typed codes without leaking diagnostic text or retrying mutations', async () => {
  let calls = 0;
  const transport = createClaimsTransport(async () => {
    calls += 1;
    return Response.json(
      { code: 'verification_timeout', detail: 'private upstream diagnostic' },
      { status: 503 },
    );
  });
  await assert.rejects(
    transport.change(claimFixture.id, 'check', new AbortController().signal),
    (error: unknown) => error instanceof ClaimProblem && error.code === 'verification_timeout',
  );
  assert.equal(calls, 1);
  assert.equal(claimMessage('verification_timeout'), '验证超时，请稍后再检查。');
  assert.equal(claimMessage('private diagnostic').includes('private'), false);
});

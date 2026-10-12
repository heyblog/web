import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { registerHooks, stripTypeScriptTypes } from 'node:module';
import test from 'node:test';

import { compileModule } from 'svelte/compiler';

import { createClaimsTransport } from '../src/api/site-management/claims.browser.ts';

import {
  claimFixture,
  contextFixture,
  ownershipFixture,
  proofFixture,
} from './site-claims.fixture.ts';
const sources = [
  '../src/application/site-management/claim-create.controller.svelte.ts',
  '../src/application/site-management/claim-detail.controller.svelte.ts',
  '../src/application/site-management/claim-review.controller.svelte.ts',
  '../src/application/site-submission/site-search.controller.svelte.ts',
];
const compiled = new Map<string, string>();
for (const path of sources) {
  const url = new URL(path, import.meta.url);
  compiled.set(
    url.href,
    compileModule(stripTypeScriptTypes(await readFile(url, 'utf8')), {
      filename: url.pathname,
      generate: 'client',
    }).js.code,
  );
}
const hooks = registerHooks({
  load(url, context, next) {
    const source = compiled.get(url);
    return source ? { source, format: 'module', shortCircuit: true } : next(url, context);
  },
});
const { createClaimCreator } =
  await import('../src/application/site-management/claim-create.controller.svelte.ts');
const { createClaimDetail } =
  await import('../src/application/site-management/claim-detail.controller.svelte.ts');
const { createClaimReview } =
  await import('../src/application/site-management/claim-review.controller.svelte.ts');
const { createSiteSearch } =
  await import('../src/application/site-submission/site-search.controller.svelte.ts');
hooks.deregister();

test('clearing a selected site ignores a late context response and prevents submission for the old site', async () => {
  let resolve: (response: Response) => void = () => {};
  let calls = 0;
  const creator = createClaimCreator(
    createClaimsTransport(() => {
      calls += 1;
      return new Promise<Response>((done) => {
        resolve = done;
      });
    }),
  );
  const selecting = creator.select(claimFixture.short_id);
  creator.clear();
  resolve(Response.json(contextFixture));
  await selecting;
  await creator.create({
    method: 'DNS_TXT',
    newAddress: false,
    address: '',
    evidence: '',
    evidenceURL: '',
  });
  assert.equal(creator.context, null);
  assert.equal(calls, 1);
  creator.dispose();
});
test('a duplicate pending creation enters existing detail without requiring a new token', async () => {
  const creator = createClaimCreator(
    createClaimsTransport(async (path, options) => {
      if (options?.method === 'POST') {
        assert.equal(path, 'site-claims');
        return Response.json({ claim: claimFixture });
      }
      return Response.json(contextFixture);
    }),
  );
  await creator.select(claimFixture.short_id);
  await creator.create({
    method: 'DNS_TXT',
    newAddress: false,
    address: 'ignored',
    evidence: '',
    evidenceURL: '',
  });
  assert.equal(creator.created?.claim.id, claimFixture.id);
  assert.equal(creator.created?.token, undefined);
  creator.dispose();
});
test('refresh resumes an existing claim without regenerating proof and expiry clears obsolete instructions', async () => {
  const calls: string[] = [];
  const detail = createClaimDetail(
    claimFixture,
    proofFixture,
    createClaimsTransport(async (path, options) => {
      calls.push(`${options?.method ?? 'GET'} ${path}`);
      return Response.json({ ...claimFixture, expires_at: '2099-10-03T00:00:00Z' });
    }),
  );
  await detail.run('refresh');
  assert.equal(detail.challenge, undefined);
  assert.deepEqual(calls, [`GET site-claims/${claimFixture.id}`]);
  detail.dispose();
});
test('explicit regeneration replaces proof in memory and disposal clears it', async () => {
  const detail = createClaimDetail(
    claimFixture,
    proofFixture,
    createClaimsTransport(async (path, options) => {
      assert.ok(path.endsWith('/regenerate'));
      assert.equal(options?.method, 'POST');
      return Response.json({ ...proofFixture, token: 'new-proof' });
    }),
  );
  await detail.run('regenerate');
  assert.equal(detail.challenge?.token, 'new-proof');
  detail.dispose();
  assert.equal(detail.challenge, undefined);
});
test('ownership revocation sends the reviewed identity and revision then reloads the whole detail', async () => {
  const calls: string[] = [];
  const review = createClaimReview(
    { ...claimFixture, status: 'VERIFIED' },
    ownershipFixture,
    createClaimsTransport(async (path, options) => {
      calls.push(`${options?.method ?? 'GET'} ${path}`);
      if (options?.method === 'DELETE') {
        assert.deepEqual(options.body, {
          ownership_id: ownershipFixture.current.id,
          revision: 2,
          reason: 'reason',
          evidence: 'evidence',
        });
        return Response.json({ success: true });
      }
      return Response.json(
        path.startsWith('site-ownership')
          ? { ...ownershipFixture, current: null, can_revoke: false }
          : { ...claimFixture, status: 'VERIFIED', can_manage: false },
      );
    }),
  );
  await review.revoke({ reason: ' reason ', evidence: ' evidence ' });
  assert.equal(review.ownership?.current, null);
  assert.equal(review.claim?.can_manage, false);
  assert.equal(calls.length, 3);
  review.dispose();
});
test('search clears stale results immediately and ignores out-of-order responses', async () => {
  const responses: ((response: Response) => void)[] = [];
  const search = createSiteSearch(
    () => new Promise<Response>((resolve) => responses.push(resolve)),
  );
  const first = search.search('first');
  const second = search.search('second');
  responses[1]?.(
    Response.json({
      items: [
        { short_id: 'second123', name: 'Second', url: 'https://second.test', visibility: 'PUBLIC' },
      ],
    }),
  );
  await second;
  responses[0]?.(
    Response.json({
      items: [
        { short_id: 'first1234', name: 'First', url: 'https://first.test', visibility: 'PUBLIC' },
      ],
    }),
  );
  await first;
  assert.equal(search.results[0]?.name, 'Second');
  search.clear();
  assert.equal(search.results.length, 0);
  search.dispose();
});

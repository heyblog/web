import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { registerHooks, stripTypeScriptTypes } from 'node:module';
import test from 'node:test';

import { compileModule } from 'svelte/compiler';

import { createClaimsTransport } from '../src/api/site-management/claims.browser.ts';
import { checkSiteAvailability } from '../src/application/site-submission/site-submission.api.browser.ts';

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
  '../src/application/site-submission/site-availability.controller.svelte.ts',
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
test('an occupied new address prevents creation of verification instructions', async () => {
  // Given: an owner can verify addresses but another site owns the new address.
  let writes = 0;
  const creator = createClaimCreator(
    createClaimsTransport(async (_path, options) => {
      if (options?.method === 'POST') writes += 1;
      return Response.json({ ...contextFixture, can_verify_address: true });
    }),
    (url, options) =>
      checkSiteAvailability(url, {
        ...options,
        fetch: async () =>
          Response.json({
            available: false,
            existing_site: {
              short_id: 'Z9y8X7w6V',
              name: 'Other',
              url: 'https://other.test/',
              visibility: 'VISIBLE',
            },
          }),
      }),
  );
  await creator.select(claimFixture.short_id);
  // When: the owner asks to verify the occupied address.
  await creator.create({
    method: 'DNS_TXT',
    newAddress: true,
    address: 'https://other.test/',
    evidence: '',
    evidenceURL: '',
  });
  // Then: no verification mutation was issued and the conflict is actionable.
  assert.equal(writes, 0);
  assert.equal(creator.created, null);
  assert.equal(creator.availability.state, 'duplicate');
  creator.dispose();
});

test('ordinary ownership claims do not perform homepage availability checks', async () => {
  // Given: an unowned site can be claimed using its existing address.
  let checks = 0;
  const creator = createClaimCreator(
    createClaimsTransport(async (_path, options) =>
      Response.json(options?.method === 'POST' ? { claim: claimFixture } : contextFixture),
    ),
    async () => {
      checks += 1;
      return { available: true };
    },
  );
  await creator.select(claimFixture.short_id);
  // When: ordinary ownership verification is requested.
  await creator.create({
    method: 'DNS_TXT',
    newAddress: false,
    address: '',
    evidence: '',
    evidenceURL: '',
  });
  // Then: only ownership semantics apply; availability was not queried.
  assert.equal(checks, 0);
  assert.equal(creator.created?.claim.id, claimFixture.id);
  creator.dispose();
});

test('quick certification returns the pending claim while keeping its popup owner mounted', async () => {
  const claim = {
    ...claimFixture,
    method: 'BLOGSCLUB',
    can_regenerate: false,
    expires_at: undefined,
  };
  const creator = createClaimCreator(
    createClaimsTransport(async (_path, options) =>
      Response.json(options?.method === 'POST' ? { claim } : contextFixture),
    ),
  );
  await creator.select(claimFixture.short_id);
  const result = await creator.create(
    { method: 'BLOGSCLUB', newAddress: false, address: '', evidence: '', evidenceURL: '' },
    false,
  );
  assert.equal(result?.claim.id, claimFixture.id);
  assert.equal(result?.claim.method, 'BLOGSCLUB');
  assert.equal(result?.token, undefined);
  assert.equal(creator.created, null);
  assert.equal(creator.busy, false);
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

test('a multi-file selection uploads every file serially within the limits', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => new Response(null, { status: 200 }));
  let initialized = 0;
  const creator = createClaimCreator(
    createClaimsTransport(async (path) => {
      if (path === 'site-claim-attachments') {
        initialized += 1;
        return Response.json({
          attachment: {
            id: String(initialized),
            filename: 'proof.pdf',
            content_type: 'application/pdf',
            size: 5,
            status: 'UPLOADING',
          },
          upload_url: 'https://bucket.cos.region.myqcloud.com/key',
          upload_headers: {},
          expires_at: '2099-01-01',
          max_size: 10485760,
          content_type: 'application/pdf',
        });
      }
      if (path.endsWith('/complete'))
        return Response.json({
          id: path.split('/')[1],
          filename: 'proof.pdf',
          content_type: 'application/pdf',
          size: 5,
          status: 'READY',
        });
      return Response.json(contextFixture);
    }),
    undefined,
    'https://bucket.cos.region.myqcloud.com',
  );
  await creator.select(claimFixture.short_id);
  await creator.addAttachments(
    [1, 2, 3].map((n) => new File(['proof'], `${n}.pdf`, { type: 'application/pdf' })),
  );
  assert.equal(initialized, 3);
  assert.equal(creator.attachments.length, 3);
  assert.equal(creator.busy, false);
  creator.dispose();
});

test('an old upload cannot clear the busy state or error of a newly selected site', async () => {
  let rejectUpload: (error: Error) => void = () => {};
  let resolveContext: (response: Response) => void = () => {};
  let reads = 0;
  const creator = createClaimCreator(
    createClaimsTransport((path) => {
      if (path === 'site-claim-attachments')
        return new Promise<Response>((_, reject) => {
          rejectUpload = reject;
        });
      reads += 1;
      if (reads === 1) return Promise.resolve(Response.json(contextFixture));
      return new Promise<Response>((resolve) => {
        resolveContext = resolve;
      });
    }),
  );
  await creator.select(claimFixture.short_id);
  const uploading = creator.addAttachment(
    new File(['proof'], 'proof.pdf', { type: 'application/pdf' }),
  );
  const selecting = creator.select('other1234');
  rejectUpload(new Error('old upload cancelled'));
  await uploading;
  assert.equal(creator.busy, true);
  assert.equal(creator.attachmentError, '');
  resolveContext(
    Response.json({ ...contextFixture, site: { ...contextFixture.site, short_id: 'other1234' } }),
  );
  await selecting;
  assert.equal(creator.attachments.length, 0);
  creator.dispose();
});

import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { registerHooks, stripTypeScriptTypes } from 'node:module';
import test, { afterEach, beforeEach } from 'node:test';

import { compileModule } from 'svelte/compiler';

import { createClaimsTransport } from '../src/api/site-management/claims.browser.ts';

import { claimFixture, proofFixture } from './site-claims.fixture.ts';

const url = new URL(
  '../src/application/site-management/claim-detail.controller.svelte.ts',
  import.meta.url,
);
const source = compileModule(stripTypeScriptTypes(await readFile(url, 'utf8')), {
  filename: url.pathname,
  generate: 'client',
}).js.code;
const hooks = registerHooks({
  load(path, context, next) {
    return path === url.href
      ? { source, format: 'module', shortCircuit: true }
      : next(path, context);
  },
});
const { createClaimDetail } =
  await import('../src/application/site-management/claim-detail.controller.svelte.ts');
hooks.deregister();

const claim = { ...claimFixture, method: 'FILE' } as const;
const task = {
  id: '11111111-1111-4111-8111-111111111111',
  state: 'queued',
  status_url: `/api/web/account/site-claims/${claim.id}/task`,
  reason: '',
};

const originalDocument = Object.getOwnPropertyDescriptor(globalThis, 'document');
beforeEach(() => {
  Object.defineProperty(globalThis, 'document', { value: { hidden: false }, configurable: true });
});
afterEach(() => {
  if (originalDocument) Object.defineProperty(globalThis, 'document', originalDocument);
  else Reflect.deleteProperty(globalThis, 'document');
});

test('an accepted HTTP check remains pending until its task verifies the claim', async () => {
  // Given the applicant check endpoint accepts a queued verification.
  const calls: string[] = [];
  const detail = createClaimDetail(
    claim,
    undefined,
    createClaimsTransport(async (path) => {
      calls.push(path);
      if (path.endsWith('/check')) return Response.json({ ...claim, task }, { status: 202 });
      if (path.endsWith('/task')) return Response.json({ ...task, state: 'succeeded' });
      return Response.json({ ...claim, status: 'VERIFIED' });
    }),
  );
  await detail.run('check');
  assert.equal(detail.checking, true);
  assert.equal(detail.claim?.status, 'PENDING');
  // When the completed task and authoritative claim are read.
  await detail.poll();
  // Then polling ends without another check mutation.
  assert.equal(detail.checking, false);
  assert.equal(detail.claim?.status, 'VERIFIED');
  assert.equal(calls.filter((path) => path.endsWith('/check')).length, 1);
  detail.dispose();
});

test('challenge regeneration waits for an old poll before replacing task state', async () => {
  // Given an old task response is in flight.
  const pending = Promise.withResolvers<Response>();
  let writes = 0;
  const renewed = { ...claim, expires_at: '2099-10-04T00:00:00Z' };
  const detail = createClaimDetail(
    { ...claim, task },
    undefined,
    createClaimsTransport(async (path) => {
      if (path.endsWith('/task')) return pending.promise;
      if (path.endsWith('/regenerate')) {
        writes++;
        return Response.json({ ...proofFixture, claim: renewed, token: 'renewed-proof' });
      }
      return Response.json(claim);
    }),
  );
  const polling = detail.poll();
  // When the applicant regenerates before that response arrives.
  const regenerating = detail.run('regenerate');
  assert.equal(detail.busy, 'regenerate');
  assert.equal(writes, 0);
  pending.resolve(Response.json({ ...task, state: 'succeeded' }));
  await polling;
  await regenerating;
  // Then no stale task survives the new challenge.
  assert.equal(writes, 1);
  assert.equal(detail.task, null);
  assert.equal(detail.claim?.expires_at, renewed.expires_at);
  assert.equal(detail.challenge?.token, 'renewed-proof');
  detail.dispose();
});

test('a proof mismatch stops polling and permits another check', async () => {
  // Given an accepted check whose proof does not match.
  let checks = 0;
  let polls = 0;
  const detail = createClaimDetail(
    claim,
    undefined,
    createClaimsTransport(async (path) => {
      if (path.endsWith('/check')) {
        checks++;
        return Response.json({ ...claim, task }, { status: 202 });
      }
      if (path.endsWith('/task')) {
        polls++;
        return Response.json({ ...task, state: 'succeeded', reason: 'proof_mismatch' });
      }
      return Response.json(claim);
    }),
  );
  await detail.run('check');
  // When the terminal decision is received.
  await detail.poll();
  // Then the polling guard is false and checking becomes available again.
  assert.equal(detail.checking, false);
  assert.equal(detail.busy, null);
  assert.equal(detail.claim?.status, 'PENDING');
  assert.equal(detail.task?.reason, 'proof_mismatch');
  assert.equal(polls, 1);
  await detail.run('check');
  assert.equal(checks, 2);
  assert.equal(detail.checking, true);
  detail.dispose();
});

test('a late regeneration response cannot restore proof after leaving the page', async () => {
  // Given regeneration has reached the HTTP boundary.
  const pending = Promise.withResolvers<Response>();
  const started = Promise.withResolvers<void>();
  const detail = createClaimDetail(
    claim,
    undefined,
    createClaimsTransport(async () => {
      started.resolve();
      return pending.promise;
    }),
  );
  const regenerating = detail.run('regenerate');
  await started.promise;
  // When the page is disposed before the response arrives.
  detail.dispose();
  pending.resolve(Response.json({ ...proofFixture, claim, token: 'late-proof' }));
  await regenerating;
  // Then the secret does not return to state.
  assert.equal(detail.challenge, undefined);
  assert.equal(detail.notice, '');
});

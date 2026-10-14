import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { registerHooks, stripTypeScriptTypes } from 'node:module';
import test from 'node:test';

import { checkSiteAvailability } from '../src/application/site-submission/site-submission.api.browser.ts';

const url = new URL(
  '../src/application/site-submission/site-availability.controller.svelte.ts',
  import.meta.url,
);
const { compileModule } = await import('svelte/compiler');
const compiled = compileModule(stripTypeScriptTypes(await readFile(url, 'utf8')), {
  filename: url.pathname,
  generate: 'client',
}).js.code;
const hooks = registerHooks({
  load(path, context, next) {
    return path === url.href
      ? { source: compiled, format: 'module', shortCircuit: true }
      : next(path, context);
  },
});
const { createSiteAvailability } =
  await import('../src/application/site-submission/site-availability.controller.svelte.ts');
hooks.deregister();
const address = { url: 'https://new.test/', currentSiteShortID: 'A1b2C3d4E' };
const existing = {
  short_id: 'B1b2C3d4F',
  name: 'Other',
  url: 'https://other.test/',
  visibility: 'VISIBLE',
};

test('a forced submission check discovers an address occupied after the first step', async () => {
  // Given: the first check succeeds and the address is occupied before submission.
  let reads = 0;
  const controller = createSiteAvailability((url, options) =>
    checkSiteAvailability(url, {
      ...options,
      fetch: async () => {
        reads += 1;
        return Response.json(
          reads === 1 ? { available: true } : { available: false, existing_site: existing },
        );
      },
    }),
  );
  assert.equal(await controller.check(address), true);
  assert.equal(await controller.check(address), true);
  assert.equal(reads, 1);
  // When: final submission forces a fresh check.
  const result = await controller.check(address, true);
  // Then: submission is blocked with recovery data instead of trusting the cache.
  assert.equal(result, false);
  assert.equal(reads, 2);
  assert.equal(controller.state, 'duplicate');
  assert.deepEqual(controller.existingSite, existing);
  controller.dispose();
});

test('editing the address cancels a request and ignores its late response', async () => {
  // Given: a check is awaiting its transport response.
  let signal: AbortSignal | undefined;
  const response = Promise.withResolvers<Response>();
  const controller = createSiteAvailability((url, options) =>
    checkSiteAvailability(url, {
      ...options,
      fetch: (_input, init) => {
        signal = init?.signal ?? undefined;
        return response.promise;
      },
    }),
  );
  const checking = controller.check(address);
  // When: the user edits the address before a conflicting response arrives.
  controller.setContext({ ...address, url: 'https://edited.test/' });
  response.resolve(Response.json({ available: false, existing_site: existing }));
  // Then: the old result cannot mark the new address as duplicate.
  assert.equal(await checking, false);
  assert.equal(signal?.aborted, true);
  assert.equal(controller.state, 'idle');
  assert.equal(controller.existingSite, null);
  controller.dispose();
});

test('switching target sites invalidates a cached self-address result', async () => {
  // Given: the address belongs to the initially selected site.
  let reads = 0;
  const owner = { ...existing, short_id: address.currentSiteShortID };
  const controller = createSiteAvailability((url, options) =>
    checkSiteAvailability(url, {
      ...options,
      fetch: async () => {
        reads += 1;
        return Response.json({ available: false, existing_site: owner });
      },
    }),
  );
  assert.equal(await controller.check(address), true);
  // When: another site is selected while the URL remains the same.
  const result = await controller.check({ ...address, currentSiteShortID: 'Z9y8X7w6V' });
  // Then: the former self-address is now a conflict and was queried again.
  assert.equal(result, false);
  assert.equal(reads, 2);
  assert.equal(controller.existingSite?.short_id, owner.short_id);
  controller.dispose();
});

test('failed checks block progression and can recover through explicit retry', async () => {
  // Given: the availability service initially fails.
  let failed = true;
  const controller = createSiteAvailability((url, options) =>
    checkSiteAvailability(url, {
      ...options,
      fetch: async () =>
        failed
          ? Response.json({ code: 'bad_gateway' }, { status: 503 })
          : Response.json({ available: true }),
    }),
  );
  assert.equal(await controller.check(address), false);
  assert.equal(controller.state, 'failed');
  failed = false;
  // When: the user retries after service recovery.
  const result = await controller.check(address, true);
  // Then: the failed state clears and the address is available.
  assert.equal(result, true);
  assert.equal(controller.state, 'available');
  assert.equal(controller.message, '');
  controller.dispose();
});

test('malformed availability success responses cannot authorize a submission', async () => {
  // Given: a successful HTTP response lacks a conflicting site identity.
  const controller = createSiteAvailability((url, options) =>
    checkSiteAvailability(url, {
      ...options,
      fetch: async () => Response.json({ available: false }),
    }),
  );
  // When: the controller checks the address.
  const result = await controller.check(address);
  // Then: the result is treated as unavailable service evidence, with no write authorized.
  assert.equal(result, false);
  assert.equal(controller.state, 'failed');
  controller.dispose();
});

test('out-of-order checks retain only the newest result', async () => {
  // Given: two different address checks are in flight.
  const first = Promise.withResolvers<Response>();
  const second = Promise.withResolvers<Response>();
  let count = 0;
  const controller = createSiteAvailability((url, options) =>
    checkSiteAvailability(url, {
      ...options,
      fetch: () => (++count === 1 ? first.promise : second.promise),
    }),
  );
  const older = controller.check(address);
  const newer = controller.check({ ...address, url: 'https://latest.test/' });
  // When: the latest succeeds before the old conflict arrives.
  second.resolve(Response.json({ available: true }));
  await newer;
  first.resolve(Response.json({ available: false, existing_site: existing }));
  await older;
  // Then: the latest address stays available.
  assert.equal(controller.state, 'available');
  assert.equal(controller.existingSite, null);
  controller.dispose();
});

test('disposal cancels the outstanding check without displaying an error', async () => {
  // Given: a controller is checking while navigation begins.
  const response = Promise.withResolvers<Response>();
  let signal: AbortSignal | undefined;
  const controller = createSiteAvailability((url, options) =>
    checkSiteAvailability(url, {
      ...options,
      fetch: (_input, init) => {
        signal = init?.signal ?? undefined;
        return response.promise;
      },
    }),
  );
  const checking = controller.check(address);
  // When: the owning page disposes its controller.
  controller.dispose();
  response.resolve(Response.json({ available: true }));
  // Then: no stale success is accepted.
  assert.equal(await checking, false);
  assert.equal(signal?.aborted, true);
  assert.equal(controller.state, 'idle');
});

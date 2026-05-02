import { afterEach, describe, expect, it } from 'vitest';

import { createTestApp } from '@tests/create-test-app';

describe('health routes', () => {
  let app: ReturnType<typeof createTestApp> | undefined;

  afterEach(async () => {
    await app?.close();
    app = undefined;
  });

  it('returns plugin summary via fastify.inject', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    const response = await app.inject({ method: 'GET', url: '/health' });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({
      ok: true,
      service: '@zhblogs/api',
      environment: 'test',
      plugins: {
        config: true,
        drizzle: true,
        cache: true,
        security: true,
      },
    });
  });

  it('skips dependency probes when external services are disabled', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    const dbResponse = await app.inject({ method: 'GET', url: '/health/db' });
    const cacheResponse = await app.inject({ method: 'GET', url: '/health/cache' });

    expect(dbResponse.statusCode).toBe(200);
    expect(dbResponse.json()).toEqual({
      ok: true,
      service: 'database',
      check: 'skipped',
    });

    expect(cacheResponse.statusCode).toBe(200);
    expect(cacheResponse.json()).toEqual({
      ok: true,
      service: 'cache',
      check: 'skipped',
    });
  });

  it('returns ready when mocked database and cache checks succeed', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.bootstrapOptions.disableExternalServices = false;
    const cache = app.db.cache as NonNullable<typeof app.db.cache>;

    expect(cache).toBeDefined();

    app.db.read.execute = (async () => [
      { '?column?': 1 },
    ]) as unknown as typeof app.db.read.execute;
    cache.ping = (async () => 'PONG') as typeof cache.ping;

    const dbResponse = await app.inject({ method: 'GET', url: '/health/db' });
    const cacheResponse = await app.inject({ method: 'GET', url: '/health/cache' });

    expect(dbResponse.statusCode).toBe(200);
    expect(dbResponse.json()).toEqual({
      ok: true,
      service: 'database',
      check: 'ready',
    });

    expect(cacheResponse.statusCode).toBe(200);
    expect(cacheResponse.json()).toEqual({
      ok: true,
      service: 'cache',
      check: 'ready',
    });
  });

  it('returns 503 when mocked database and cache checks fail', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.bootstrapOptions.disableExternalServices = false;
    const cache = app.db.cache as NonNullable<typeof app.db.cache>;

    expect(cache).toBeDefined();

    app.db.read.execute = (async () => {
      throw new Error('db unavailable');
    }) as unknown as typeof app.db.read.execute;
    cache.ping = (async () => {
      throw new Error('cache unavailable');
    }) as typeof cache.ping;

    const dbResponse = await app.inject({ method: 'GET', url: '/health/db' });
    const cacheResponse = await app.inject({ method: 'GET', url: '/health/cache' });

    expect(dbResponse.statusCode).toBe(503);
    expect(dbResponse.json()).toEqual({
      ok: false,
      service: 'database',
      check: 'failed',
    });

    expect(cacheResponse.statusCode).toBe(503);
    expect(cacheResponse.json()).toEqual({
      ok: false,
      service: 'cache',
      check: 'failed',
    });
  });

  it('accepts health requests that include an origin header', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    const response = await app.inject({
      method: 'GET',
      url: '/health',
      headers: {
        origin: 'http://127.0.0.1:9101',
      },
    });

    expect(response.statusCode).toBe(200);
  });
});

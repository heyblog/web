import { afterEach, describe, expect, it } from 'vitest';

import { createTestApp } from '@tests/create-test-app';

describe('presence routes', () => {
  let app: ReturnType<typeof createTestApp> | undefined;

  afterEach(async () => {
    await app?.close();
    app = undefined;
  });

  it('returns the current online count with timing metadata', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    const response = await app.inject({
      method: 'GET',
      url: '/api/presence/online',
      headers: {
        origin: 'http://127.0.0.1:9101',
      },
    });

    expect(response.statusCode).toBe(200);
    expect(response.headers['access-control-allow-origin']).toBe('http://127.0.0.1:9101');
    expect(response.json()).toEqual({
      ok: true,
      data: {
        count: 0,
        activeWindowMs: 45000,
        heartbeatIntervalMs: 15000,
      },
    });
  });

  it('tracks unique heartbeats without double counting the same client', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    const firstHeartbeat = await app.inject({
      method: 'POST',
      url: '/api/presence/heartbeat',
      payload: {
        clientId: '11111111-1111-4111-8111-111111111111',
      },
    });

    const secondHeartbeat = await app.inject({
      method: 'POST',
      url: '/api/presence/heartbeat',
      payload: {
        clientId: '11111111-1111-4111-8111-111111111111',
      },
    });

    const thirdHeartbeat = await app.inject({
      method: 'POST',
      url: '/api/presence/heartbeat',
      payload: {
        clientId: '22222222-2222-4222-8222-222222222222',
      },
    });

    expect(firstHeartbeat.statusCode).toBe(200);
    expect(firstHeartbeat.json()).toEqual({
      ok: true,
      data: {
        count: 1,
      },
    });
    expect(secondHeartbeat.statusCode).toBe(200);
    expect(secondHeartbeat.json()).toEqual({
      ok: true,
      data: {
        count: 1,
      },
    });
    expect(thirdHeartbeat.statusCode).toBe(200);
    expect(thirdHeartbeat.json()).toEqual({
      ok: true,
      data: {
        count: 2,
      },
    });
  });

  it('rejects malformed heartbeat payloads', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    const response = await app.inject({
      method: 'POST',
      url: '/api/presence/heartbeat',
      payload: {
        clientId: 'not-a-uuid',
      },
    });

    expect(response.statusCode).toBe(400);
    expect(response.json()).toEqual({
      ok: false,
      message: 'clientId must be a valid UUID.',
    });
  });
});

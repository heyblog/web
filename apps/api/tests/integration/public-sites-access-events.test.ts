import { SiteAccessEvents, Sites } from '@zhblogs/db';

import { afterEach, describe, expect, it, vi } from 'vitest';

import { createTestApp } from '@tests/create-test-app';

describe('public site access event route', () => {
  let app: ReturnType<typeof createTestApp> | undefined;

  afterEach(async () => {
    vi.restoreAllMocks();
    await app?.close();
    app = undefined;
  });

  it('records outbound click events for public sites', async () => {
    app = createTestApp({ disableExternalServices: true });
    await app.ready();

    app.db.read.select = vi.fn(() => ({
      from(table: unknown) {
        expect(table).toBe(Sites);

        return {
          where: vi.fn(() => ({
            limit: vi.fn(async () => [{ id: 'site-1' }]),
          })),
        };
      },
    })) as unknown as typeof app.db.read.select;

    let insertedValues: unknown;
    app.db.write.insert = vi.fn((table: unknown) => {
      expect(table).toBe(SiteAccessEvents);

      return {
        values: vi.fn(async (values) => {
          insertedValues = values;
        }),
      };
    }) as unknown as typeof app.db.write.insert;

    const response = await app.inject({
      method: 'POST',
      url: '/api/public/sites/4ffb6f48-63d1-4e3a-9c36-b9fc86dd65c0/access-events',
      headers: {
        'content-type': 'application/json',
        referer: 'https://www.zhblogs.net/site/go?recommend=true',
        origin: 'https://www.zhblogs.net',
        'user-agent': 'vitest',
      },
      payload: {
        source: 'SITE_GO',
        targetKind: 'SITE',
        path: '/site/go?recommend=true',
      },
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({
      ok: true,
      data: { recorded: true },
    });
    expect(insertedValues).toEqual({
      site_id: '4ffb6f48-63d1-4e3a-9c36-b9fc86dd65c0',
      event_type: 'OUTBOUND_CLICK',
      source: 'SITE_GO:SITE',
      referer_host: 'www.zhblogs.net',
      path: '/site/go?recommend=true',
      user_agent: 'vitest',
    });
  });

  it('rejects invalid request bodies', async () => {
    app = createTestApp({ disableExternalServices: true });
    await app.ready();

    const response = await app.inject({
      method: 'POST',
      url: '/api/public/sites/4ffb6f48-63d1-4e3a-9c36-b9fc86dd65c0/access-events',
      headers: {
        'content-type': 'application/json',
      },
      payload: {
        source: 'SITE_GO',
        targetKind: 'SITE',
        path: '',
      },
    });

    expect(response.statusCode).toBe(400);
    expect(response.json().error.code).toBe('INVALID_BODY');
  });

  it('returns 404 when the site is not public', async () => {
    app = createTestApp({ disableExternalServices: true });
    await app.ready();

    app.db.read.select = vi.fn(() => ({
      from(table: unknown) {
        expect(table).toBe(Sites);

        return {
          where: vi.fn(() => ({
            limit: vi.fn(async () => []),
          })),
        };
      },
    })) as unknown as typeof app.db.read.select;

    const response = await app.inject({
      method: 'POST',
      url: '/api/public/sites/4ffb6f48-63d1-4e3a-9c36-b9fc86dd65c0/access-events',
      headers: {
        'content-type': 'application/json',
      },
      payload: {
        source: 'SITE_DETAIL',
        targetKind: 'ARTICLE',
        path: '/site/4ffb6f48-63d1-4e3a-9c36-b9fc86dd65c0',
      },
    });

    expect(response.statusCode).toBe(404);
    expect(response.json().error.code).toBe('SITE_NOT_FOUND');
  });
});

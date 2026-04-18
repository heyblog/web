import { afterEach, describe, expect, it, vi } from 'vitest';

import { createTestApp } from '@tests/create-test-app';

import { mockRandomQueries } from './public-sites-random.helpers';

describe('public site random route', () => {
  let app: ReturnType<typeof createTestApp> | undefined;

  afterEach(async () => {
    vi.restoreAllMocks();
    await app?.close();
    app = undefined;
  });

  it('returns only safe sites when no filter is provided', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    mockRandomQueries(app, {
      mainTags: [
        { id: 'main-1', name: '技术', machineKey: null, tagType: 'MAIN' },
        { id: 'main-2', name: '生活', machineKey: null, tagType: 'MAIN' },
      ],
      sites: [
        {
          id: 'site-safe',
          bid: 'safe',
          name: 'Safe Site',
          url: 'https://safe.example',
          sign: '安全站点',
          feeds: [{ url: 'https://safe.example/feed.xml', isDefault: true }],
          sitemap: null,
          linkPage: null,
          featured: false,
          status: 'OK',
          accessScope: 'ALL',
          joinTime: new Date('2026-03-01T08:00:00.000Z'),
          updateTime: new Date('2026-03-25T08:00:00.000Z'),
          reason: null,
        },
        {
          id: 'site-down',
          bid: 'down',
          name: 'Down Site',
          url: 'https://down.example',
          sign: '异常站点',
          feeds: [],
          sitemap: null,
          linkPage: null,
          featured: true,
          status: 'DOWN',
          accessScope: 'ALL',
          joinTime: new Date('2026-03-02T08:00:00.000Z'),
          updateTime: new Date('2026-03-20T08:00:00.000Z'),
          reason: null,
        },
        {
          id: 'site-warn',
          bid: 'warn',
          name: 'Warn Site',
          url: 'https://warn.example',
          sign: '告警站点',
          feeds: [],
          sitemap: null,
          linkPage: null,
          featured: false,
          status: 'OK',
          accessScope: 'ALL',
          joinTime: new Date('2026-03-03T08:00:00.000Z'),
          updateTime: new Date('2026-03-18T08:00:00.000Z'),
          reason: null,
        },
      ],
      stats: [
        {
          site_id: 'site-safe',
          visible_articles: 12,
          total_articles: 12,
          latest_published_time: new Date('2026-03-24T08:00:00.000Z'),
        },
      ],
      access: [{ site_id: 'site-safe', total: 80 }],
      tags: [
        { site_id: 'site-safe', tagName: '技术', tagType: 'MAIN' },
        { site_id: 'site-warn', tagName: '生活', tagType: 'MAIN' },
      ],
      warnings: [
        {
          siteId: 'site-warn',
          source: 'MANUAL',
          note: 'warn',
          createdTime: new Date('2026-03-24T08:00:00.000Z'),
          id: 'warning-1',
          machineKey: 'EXTERNAL_LIMIT',
          name: '外部限制',
          description: 'warning',
        },
      ],
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/public/sites/random',
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({
      ok: true,
      data: expect.objectContaining({
        failureReason: null,
        filters: { recommend: false, type: '' },
        availableTypes: ['技术', '生活'],
        site: expect.objectContaining({
          id: 'site-safe',
          name: 'Safe Site',
          status: 'OK',
          warningTags: [],
        }),
      }),
    });
  });

  it('returns UNKNOWN_PARAM for unsupported query keys', async () => {
    app = createTestApp({ disableExternalServices: true });
    await app.ready();

    mockRandomQueries(app, {
      mainTags: [{ id: 'main-1', name: '技术', machineKey: null, tagType: 'MAIN' }],
      sites: [],
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/public/sites/random?foo=bar',
    });

    expect(response.statusCode).toBe(200);
    expect(response.json().data.failureReason).toBe('UNKNOWN_PARAM');
    expect(response.json().data.site).toBeNull();
  });

  it('returns INVALID_RECOMMEND for unsupported recommend values', async () => {
    app = createTestApp({ disableExternalServices: true });
    await app.ready();

    mockRandomQueries(app, {
      mainTags: [{ id: 'main-1', name: '技术', machineKey: null, tagType: 'MAIN' }],
      sites: [],
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/public/sites/random?recommend=false',
    });

    expect(response.statusCode).toBe(200);
    expect(response.json().data.failureReason).toBe('INVALID_RECOMMEND');
  });

  it('returns INVALID_TYPE for unknown main tags', async () => {
    app = createTestApp({ disableExternalServices: true });
    await app.ready();

    mockRandomQueries(app, {
      mainTags: [{ id: 'main-1', name: '技术', machineKey: null, tagType: 'MAIN' }],
      sites: [],
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/public/sites/random?type=%E8%AE%BE%E8%AE%A1',
    });

    expect(response.statusCode).toBe(200);
    expect(response.json().data.failureReason).toBe('INVALID_TYPE');
  });

  it('returns INVALID_PARAMS when recommend and type are both invalid', async () => {
    app = createTestApp({ disableExternalServices: true });
    await app.ready();

    mockRandomQueries(app, {
      mainTags: [{ id: 'main-1', name: '技术', machineKey: null, tagType: 'MAIN' }],
      sites: [],
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/public/sites/random?recommend=false&type=%E8%AE%BE%E8%AE%A1',
    });

    expect(response.statusCode).toBe(200);
    expect(response.json().data.failureReason).toBe('INVALID_PARAMS');
    expect(response.json().data.site).toBeNull();
  });

  it('returns NO_MATCH when valid filters produce no safe candidates', async () => {
    app = createTestApp({ disableExternalServices: true });
    await app.ready();

    mockRandomQueries(app, {
      mainTags: [{ id: 'main-1', name: '技术', machineKey: null, tagType: 'MAIN' }],
      sites: [
        {
          id: 'site-1',
          bid: 'site-1',
          name: 'Site 1',
          url: 'https://site-1.example',
          sign: '技术站点',
          feeds: [],
          sitemap: null,
          linkPage: null,
          featured: false,
          status: 'OK',
          accessScope: 'ALL',
          joinTime: new Date('2026-03-01T08:00:00.000Z'),
          updateTime: new Date('2026-03-25T08:00:00.000Z'),
          reason: null,
        },
      ],
      tags: [{ site_id: 'site-1', tagName: '技术', tagType: 'MAIN' }],
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/public/sites/random?recommend=true&type=%E6%8A%80%E6%9C%AF',
    });

    expect(response.statusCode).toBe(200);
    expect(response.json().data.failureReason).toBe('NO_MATCH');
    expect(response.json().data.site).toBeNull();
  });
});

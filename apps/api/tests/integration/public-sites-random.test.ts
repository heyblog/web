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
      randomSite: {
        siteId: 'site-safe',
        bid: 'safe',
        name: 'Safe Site',
        url: 'https://safe.example',
        sign: '安全站点',
        feeds: [{ url: 'https://safe.example/feed.xml', isDefault: true }],
        feedUrl: 'https://safe.example/feed.xml',
        sitemap: null,
        linkPage: null,
        featured: false,
        status: 'OK',
        accessScope: 'ALL',
        joinTime: new Date('2026-03-01T08:00:00.000Z'),
        updateTime: new Date('2026-03-25T08:00:00.000Z'),
        reason: null,
        articleCount: 12,
        latestPublishedTime: new Date('2026-03-24T08:00:00.000Z'),
        visitCount: 80,
        primaryTag: '技术',
        subTags: [],
        warningTags: [],
        warningNames: [],
        programId: null,
        programName: null,
        programIsOpenSource: null,
        websiteUrl: null,
        repoUrl: null,
      },
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

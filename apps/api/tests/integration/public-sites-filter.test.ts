import { afterEach, describe, expect, it, vi } from 'vitest';

import { createTestApp } from '@tests/create-test-app';

describe('public site directory filters', () => {
  let app: ReturnType<typeof createTestApp> | undefined;

  afterEach(async () => {
    vi.restoreAllMocks();
    await app?.close();
    app = undefined;
  });

  it('filters public site cards with structured query syntax', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.db.read.execute = vi
      .fn()
      .mockResolvedValueOnce([{ totalItems: 1 }])
      .mockResolvedValueOnce([
        {
          siteId: 'site-1',
          bid: 'alpha-bid',
          name: 'Alpha Overseas',
          url: 'https://alpha.example',
          sign: '海外可访问的技术站点。',
          feeds: [
            {
              url: 'https://alpha.example/feed.xml',
              isDefault: true,
            },
          ],
          feedUrl: 'https://alpha.example/feed.xml',
          sitemap: 'https://alpha.example/sitemap.xml',
          linkPage: null,
          featured: true,
          status: 'WARNING',
          accessScope: 'NON_CN_ONLY',
          joinTime: new Date('2026-03-01T08:00:00.000Z'),
          updateTime: new Date('2026-03-25T08:00:00.000Z'),
          reason: null,
          articleCount: 8,
          latestPublishedTime: new Date('2026-03-25T06:00:00.000Z'),
          visitCount: 88,
          primaryTag: '技术',
          subTags: [],
          warningTags: [],
          warningNames: [],
          programId: 'program-1',
          programName: 'Astro',
          programIsOpenSource: true,
          websiteUrl: 'https://astro.build',
          repoUrl: 'https://github.com/withastro/astro',
        },
      ]) as unknown as typeof app.db.read.execute;

    const response = await app.inject({
      method: 'GET',
      url: '/api/public/sites?statusMode=abnormal&q=featured:true%20rss:true%20site:Alpha%20domain:alpha.example%20access:%E6%B5%B7%E5%A4%96%20program:Astro',
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({
      ok: true,
      data: {
        items: [
          expect.objectContaining({
            id: 'site-1',
            bid: 'alpha-bid',
            name: 'Alpha Overseas',
            status: 'WARNING',
            accessScope: 'NON_CN_ONLY',
            featured: true,
            feedUrl: 'https://alpha.example/feed.xml',
          }),
        ],
        pagination: {
          page: 1,
          pageSize: 24,
          totalItems: 1,
          totalPages: 1,
        },
        query: {
          q: 'featured:true rss:true site:Alpha domain:alpha.example access:海外 program:Astro',
          main: [],
          sub: [],
          warning: [],
          program: ['Astro'],
          statusMode: 'abnormal',
          random: true,
          sort: null,
          order: 'desc',
          randomSeed: 'public-site-directory',
        },
      },
    });
  });
});

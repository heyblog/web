import {
  SiteAccessCounters,
  SiteArchitectures,
  SiteFeedArticleStats,
  Sites,
  SiteTags,
  SiteWarningTags,
  TagDefinitions,
} from '@zhblogs/db';

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

    const selectQueue = [
      {
        table: Sites,
        rows: [
          {
            id: 'site-1',
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
            sitemap: 'https://alpha.example/sitemap.xml',
            linkPage: null,
            featured: true,
            status: 'WARNING',
            accessScope: 'NON_CN_ONLY',
            joinTime: new Date('2026-03-01T08:00:00.000Z'),
            updateTime: new Date('2026-03-25T08:00:00.000Z'),
            reason: null,
          },
          {
            id: 'site-2',
            bid: 'beta-bid',
            name: 'Beta Mainland',
            url: 'https://beta.example',
            sign: '中国大陆可访问的社区站点。',
            feeds: [],
            sitemap: null,
            linkPage: null,
            featured: false,
            status: 'OK',
            accessScope: 'CN_ONLY',
            joinTime: new Date('2026-03-02T08:00:00.000Z'),
            updateTime: new Date('2026-03-20T08:00:00.000Z'),
            reason: null,
          },
        ],
      },
      {
        table: SiteFeedArticleStats,
        rows: [
          {
            site_id: 'site-1',
            visible_articles: 8,
            total_articles: 8,
            latest_published_time: new Date('2026-03-25T06:00:00.000Z'),
          },
          {
            site_id: 'site-2',
            visible_articles: 5,
            total_articles: 5,
            latest_published_time: new Date('2026-03-20T06:00:00.000Z'),
          },
        ],
      },
      {
        table: SiteAccessCounters,
        rows: [
          {
            site_id: 'site-1',
            total: 88,
          },
          {
            site_id: 'site-2',
            total: 42,
          },
        ],
      },
      {
        table: SiteTags,
        rows: [
          {
            site_id: 'site-1',
            tagName: '技术',
            tagType: 'MAIN',
          },
          {
            site_id: 'site-2',
            tagName: '社区',
            tagType: 'MAIN',
          },
        ],
      },
      {
        table: SiteWarningTags,
        rows: [],
      },
      {
        table: SiteArchitectures,
        rows: [
          {
            siteId: 'site-1',
            programId: 'program-1',
            programName: 'Astro',
          },
          {
            siteId: 'site-2',
            programId: 'program-2',
            programName: 'Hugo',
          },
        ],
      },
    ];

    app.db.read.select = vi.fn(() => ({
      from(table: unknown) {
        const next = selectQueue.shift();

        expect(next?.table).toBe(table);

        if (table === Sites) {
          return {
            where: vi.fn(() => ({
              orderBy: vi.fn(async () => next?.rows ?? []),
            })),
          };
        }

        if (table === SiteTags) {
          return {
            innerJoin: vi.fn((joinedTable: unknown) => {
              expect(joinedTable).toBe(TagDefinitions);

              return {
                where: vi.fn(async () => next?.rows ?? []),
              };
            }),
          };
        }

        if (table === SiteWarningTags) {
          return {
            innerJoin: vi.fn((joinedTable: unknown) => {
              expect(joinedTable).toBe(TagDefinitions);

              return {
                where: vi.fn(() => ({
                  orderBy: vi.fn(async () => next?.rows ?? []),
                })),
              };
            }),
          };
        }

        if (table === SiteArchitectures) {
          return {
            innerJoin: vi.fn(() => ({
              where: vi.fn(() => ({
                orderBy: vi.fn(async () => next?.rows ?? []),
              })),
            })),
          };
        }

        return {
          where: vi.fn(async () => next?.rows ?? []),
        };
      },
    })) as unknown as typeof app.db.read.select;

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

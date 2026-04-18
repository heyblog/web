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

describe('public site directory list', () => {
  let app: ReturnType<typeof createTestApp> | undefined;

  afterEach(async () => {
    vi.restoreAllMocks();
    await app?.close();
    app = undefined;
  });

  it('returns public site cards', async () => {
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
            bid: 'cloud-atlas',
            name: 'Cloud Atlas',
            url: 'https://cloud-atlas.example',
            sign: '记录基础设施与系统实验。',
            feeds: [
              {
                url: 'https://cloud-atlas.example/feed.xml',
                isDefault: true,
              },
            ],
            sitemap: 'https://cloud-atlas.example/sitemap.xml',
            linkPage: 'https://cloud-atlas.example/friends',
            featured: true,
            status: 'OK',
            accessScope: 'ALL',
            joinTime: new Date('2026-03-01T08:00:00.000Z'),
            updateTime: new Date('2026-03-25T08:00:00.000Z'),
            reason: null,
          },
        ],
      },
      {
        table: SiteFeedArticleStats,
        rows: [
          {
            site_id: 'site-1',
            visible_articles: 18,
            total_articles: 22,
            latest_published_time: new Date('2026-03-25T06:00:00.000Z'),
          },
        ],
      },
      {
        table: SiteAccessCounters,
        rows: [
          {
            site_id: 'site-1',
            total: 320,
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
            site_id: 'site-1',
            tagName: '架构',
            tagType: 'SUB',
          },
          {
            site_id: 'site-1',
            tagName: '运维',
            tagType: 'SUB',
          },
        ],
      },
      {
        table: SiteWarningTags,
        rows: [
          {
            siteId: 'site-1',
            source: 'MANUAL',
            note: '近期存在访问限制',
            createdTime: new Date('2026-03-24T08:00:00.000Z'),
            id: 'warning-tag-1',
            machineKey: 'EXTERNAL_LIMIT',
            name: '外部限制',
            description: '网站受地区限制、防火墙或外部网络策略影响',
          },
        ],
      },
      {
        table: SiteArchitectures,
        rows: [
          {
            siteId: 'site-1',
            programId: 'program-1',
            programName: 'Astro',
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
      url: '/api/public/sites',
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({
      ok: true,
      data: {
        items: [
          {
            id: 'site-1',
            bid: 'cloud-atlas',
            slug: 'site-1',
            name: 'Cloud Atlas',
            url: 'https://cloud-atlas.example',
            sign: '记录基础设施与系统实验。',
            feedUrl: 'https://cloud-atlas.example/feed.xml',
            sitemap: 'https://cloud-atlas.example/sitemap.xml',
            linkPage: 'https://cloud-atlas.example/friends',
            featured: true,
            status: 'OK',
            accessScope: 'ALL',
            joinTime: '2026-03-01T08:00:00.000Z',
            updateTime: '2026-03-25T08:00:00.000Z',
            latestPublishedTime: '2026-03-25T06:00:00.000Z',
            articleCount: 18,
            visitCount: 320,
            primaryTag: '技术',
            subTags: ['架构', '运维'],
            warningTags: [
              {
                machineKey: 'EXTERNAL_LIMIT',
                name: '外部限制',
                description: '网站受地区限制、防火墙或外部网络策略影响',
              },
            ],
          },
        ],
        pagination: {
          page: 1,
          pageSize: 24,
          totalItems: 1,
          totalPages: 1,
        },
        query: {
          q: '',
          main: [],
          sub: [],
          warning: [],
          program: [],
          statusMode: 'normal',
          random: true,
          sort: null,
          order: 'desc',
          randomSeed: 'public-site-directory',
        },
      },
    });
  });
});

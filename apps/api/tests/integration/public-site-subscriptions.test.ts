import { FeedArticles, Sites } from '@zhblogs/db';

import { afterEach, describe, expect, it, vi } from 'vitest';

import { createTestApp } from '@tests/create-test-app';

describe('public site subscriptions', () => {
  let app: ReturnType<typeof createTestApp> | undefined;

  afterEach(async () => {
    vi.restoreAllMocks();
    await app?.close();
    app = undefined;
  });

  it('returns subscription summary and paged rss articles', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    let selectCount = 0;

    app.db.read.select = vi.fn(() => ({
      from(table: unknown) {
        expect(table).toBe(FeedArticles);

        return {
          innerJoin: vi.fn((joinedTable: unknown) => {
            expect(joinedTable).toBe(Sites);
            selectCount += 1;

            if (selectCount === 1) {
              return {
                where: vi.fn(async () => [
                  {
                    todayArticles: 2,
                    weekArticles: 8,
                    totalArticles: 30,
                    siteCount: 5,
                  },
                ]),
              };
            }

            return {
              where: vi.fn(() => ({
                orderBy: vi.fn(() => ({
                  limit: vi.fn(() => ({
                    offset: vi.fn(async () => [
                      {
                        id: 'article-1',
                        articleUrl: 'https://alpha.example/posts/1',
                        title: 'Alpha Post',
                        summary: 'Alpha Summary',
                        publishedTime: new Date('2026-04-15T08:00:00.000Z'),
                        fetchedTime: new Date('2026-04-15T08:05:00.000Z'),
                        source: {
                          feed_name: 'Main Feed',
                          feed_url: 'https://alpha.example/feed.xml',
                          feed_type: 'RSS',
                        },
                        siteId: 'site-1',
                        siteBid: 'alpha',
                        siteName: 'Alpha',
                        siteUrl: 'https://alpha.example',
                      },
                    ]),
                  })),
                })),
              })),
            };
          }),
        };
      },
    })) as unknown as typeof app.db.read.select;

    const response = await app.inject({
      method: 'GET',
      url: '/api/public/subscriptions?page=2&pageSize=10',
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({
      ok: true,
      data: {
        summary: {
          todayArticles: 2,
          weekArticles: 8,
          totalArticles: 30,
          siteCount: 5,
        },
        items: [
          {
            id: 'article-1',
            title: 'Alpha Post',
            articleUrl: 'https://alpha.example/posts/1',
            summary: 'Alpha Summary',
            publishedTime: '2026-04-15T08:00:00.000Z',
            fetchedTime: '2026-04-15T08:05:00.000Z',
            source: {
              feedName: 'Main Feed',
              feedUrl: 'https://alpha.example/feed.xml',
              feedType: 'RSS',
            },
            site: {
              id: 'site-1',
              slug: 'site-1',
              name: 'Alpha',
              url: 'https://alpha.example',
            },
          },
        ],
        pagination: {
          page: 2,
          pageSize: 10,
          totalItems: 30,
          totalPages: 3,
        },
      },
    });
  });
});

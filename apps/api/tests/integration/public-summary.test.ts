import { Announcements } from '@zhblogs/db';

import { afterEach, describe, expect, it, vi } from 'vitest';

import { createTestApp } from '@tests/create-test-app';

describe('public routes summary', () => {
  let app: ReturnType<typeof createTestApp> | undefined;

  afterEach(async () => {
    vi.restoreAllMocks();
    await app?.close();
    app = undefined;
  });

  const setupApp = async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();
  };

  const getApp = () => {
    if (!app) {
      throw new Error('app not initialized');
    }

    return app;
  };

  const homeSummaryResponse = {
    ok: true,
    data: {
      summary: {
        totalSites: 12,
        featuredSites: 4,
        todayUpdates: 2,
      },
    },
  };

  const currentAnnouncementResponse = {
    ok: true,
    data: {
      id: 'announcement-1',
      title: 'Current Notice',
      content: 'Directory updates are in progress.',
      publishTime: '2026-03-20T10:00:00.000Z',
    },
  };

  const archiveResponse = {
    ok: true,
    data: {
      items: [
        {
          id: 'announcement-1',
          title: 'Spring Cleanup',
          content: 'Body 1',
          status: 'PUBLISHED',
          publishTime: '2026-03-20T10:00:00.000Z',
          expireTime: null,
        },
        {
          id: 'announcement-2',
          title: 'Submission Notice',
          content: null,
          status: 'EXPIRED',
          publishTime: '2026-03-18T10:00:00.000Z',
          expireTime: '2026-03-21T10:00:00.000Z',
        },
      ],
      pagination: {
        page: 1,
        pageSize: 20,
        totalItems: 2,
        totalPages: 1,
      },
    },
  };

  const archiveTotalRows = [{ total: 2 }];
  const archiveSelectRows = [
    {
      id: 'announcement-1',
      title: 'Spring Cleanup',
      content: 'Body 1',
      status: 'PUBLISHED',
      publishTime: new Date('2026-03-20T10:00:00.000Z'),
      expireTime: null,
    },
    {
      id: 'announcement-2',
      title: 'Submission Notice',
      content: null,
      status: 'EXPIRED',
      publishTime: new Date('2026-03-18T10:00:00.000Z'),
      expireTime: new Date('2026-03-21T10:00:00.000Z'),
    },
  ];

  const mockHomeSummarySelect = () => {
    const currentApp = getApp();
    currentApp.db.read.execute = vi.fn(async () => [
      {
        totalSites: 12,
        featuredSites: 4,
        todayUpdates: 2,
      },
    ]) as unknown as typeof currentApp.db.read.execute;
  };

  const mockCurrentAnnouncementSelect = () => {
    const currentApp = getApp();
    currentApp.db.read.select = vi.fn(() => ({
      from(table: unknown) {
        expect(table).toBe(Announcements);

        return {
          where: vi.fn(() => ({
            orderBy: vi.fn(() => ({
              limit: vi.fn(async () => [
                {
                  id: 'announcement-1',
                  title: 'Current Notice',
                  content: 'Directory updates are in progress.',
                  publishTime: new Date('2026-03-20T10:00:00.000Z'),
                },
              ]),
            })),
          })),
        };
      },
    })) as unknown as typeof currentApp.db.read.select;
  };

  const buildArchiveRowsQuery = () => ({
    where: vi.fn(() => ({
      orderBy: vi.fn(() => ({
        limit: vi.fn(() => ({
          offset: vi.fn(async () => archiveSelectRows),
        })),
      })),
    })),
  });

  const buildArchiveCountQuery = () => ({
    where: vi.fn(async () => archiveTotalRows),
  });

  const mockAnnouncementArchiveSelect = () => {
    const currentApp = getApp();
    let selectIndex = 0;
    currentApp.db.read.select = vi.fn(() => {
      selectIndex += 1;

      if (selectIndex === 1) {
        return {
          from(table: unknown) {
            expect(table).toBe(Announcements);

            return {
              ...buildArchiveCountQuery(),
            };
          },
        };
      }

      return {
        from(table: unknown) {
          expect(table).toBe(Announcements);

          return buildArchiveRowsQuery();
        },
      };
    }) as unknown as typeof currentApp.db.read.select;
  };

  const expectOkResponse = (
    response: { statusCode: number; json: () => unknown },
    expected: unknown,
  ) => {
    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual(expected);
  };

  it('returns public home summary', async () => {
    await setupApp();
    mockHomeSummarySelect();

    const response = await getApp().inject({
      method: 'GET',
      url: '/api/home',
    });

    expectOkResponse(response, homeSummaryResponse);
  });

  it('returns the current published announcement', async () => {
    await setupApp();
    mockCurrentAnnouncementSelect();

    const response = await getApp().inject({
      method: 'GET',
      url: '/api/announcements/current',
    });

    expectOkResponse(response, currentAnnouncementResponse);
  });

  it('returns paged public announcement archive', async () => {
    await setupApp();
    mockAnnouncementArchiveSelect();

    const response = await getApp().inject({
      method: 'GET',
      url: '/api/announcements?page=1&pageSize=20',
    });

    expectOkResponse(response, archiveResponse);
  });
});

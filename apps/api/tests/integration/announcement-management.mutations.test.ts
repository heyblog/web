import { Announcements } from '@zhblogs/db';

import { afterEach, describe, expect, it, vi } from 'vitest';

import { TEST_AUTH_COOKIES } from '../config';

import {
  type AnnouncementTestApp,
  baseAdminUser,
  createAnnouncementTestApp,
} from './announcement-management.shared';

describe('announcement management mutation routes', () => {
  let app: AnnouncementTestApp | undefined;

  afterEach(async () => {
    vi.restoreAllMocks();
    await app?.close();
    app = undefined;
  });

  const getApp = () => {
    if (!app) {
      throw new Error('app not initialized');
    }

    return app;
  };
  const mockOverlapSelect = (rows: Array<Record<string, unknown>>) => {
    const currentApp = getApp();
    currentApp.db.read.select = vi.fn(() => ({
      from(table: unknown) {
        expect(table).toBe(Announcements);

        return {
          where: vi.fn(async () => rows),
        };
      },
    })) as unknown as typeof currentApp.db.read.select;
  };
  const mockStatusAndOverlapSelect = (
    status: string,
    overlapRows: Array<Record<string, unknown>>,
  ) => {
    const currentApp = getApp();
    currentApp.db.read.select = vi.fn((fields?: Record<string, unknown>) => {
      if (fields && 'status' in fields) {
        return {
          from(table: unknown) {
            expect(table).toBe(Announcements);

            return {
              where: vi.fn(() => ({
                limit: vi.fn(async () => [
                  {
                    id: '11111111-1111-4111-8111-222222222222',
                    status,
                  },
                ]),
              })),
            };
          },
        };
      }

      return {
        from(table: unknown) {
          expect(table).toBe(Announcements);

          return {
            where: vi.fn(async () => overlapRows),
          };
        },
      };
    }) as unknown as typeof currentApp.db.read.select;
  };
  const mockAnnouncementUpdate = (responseRow: Record<string, unknown>) => {
    const currentApp = getApp();
    let capturedValues: Record<string, unknown> | undefined;

    currentApp.db.write.update = vi.fn((table: unknown) => {
      expect(table).toBe(Announcements);

      return {
        set: vi.fn((values: Record<string, unknown>) => {
          capturedValues = values;

          return {
            where: vi.fn(() => ({
              returning: vi.fn(async () => [responseRow]),
            })),
          };
        }),
      };
    }) as unknown as typeof currentApp.db.write.update;

    return () => capturedValues;
  };
  const mockAnnouncementInsert = (
    buildRow: (values: Record<string, unknown>) => Record<string, unknown>,
  ) => {
    const currentApp = getApp();
    let capturedValues: Record<string, unknown> | undefined;

    currentApp.db.write.insert = vi.fn((table: unknown) => {
      expect(table).toBe(Announcements);

      return {
        values: vi.fn((values: Record<string, unknown>) => {
          capturedValues = values;

          return {
            returning: vi.fn(async () => [buildRow(values)]),
          };
        }),
      };
    }) as unknown as typeof currentApp.db.write.insert;

    return () => capturedValues;
  };

  const mockAnnouncementByIdSelect = (row: Record<string, unknown>) => {
    const currentApp = getApp();
    currentApp.db.read.select = vi.fn(() => ({
      from(table: unknown) {
        expect(table).toBe(Announcements);

        return {
          where: vi.fn(() => ({
            limit: vi.fn(async () => [row]),
          })),
        };
      },
    })) as unknown as typeof currentApp.db.read.select;
  };

  const mockAnnouncementDeleteSelect = (status: string) => {
    const currentApp = getApp();
    currentApp.db.read.select = vi.fn(() => ({
      from(table: unknown) {
        expect(table).toBe(Announcements);

        return {
          where: vi.fn(() => ({
            limit: vi.fn(async () => [
              {
                id: '11111111-1111-4111-8111-222222222222',
                status,
              },
            ]),
          })),
        };
      },
    })) as unknown as typeof currentApp.db.read.select;
  };

  const updatedAnnouncementResponse = {
    ok: true,
    data: {
      id: '11111111-1111-4111-8111-222222222222',
      title: 'Current announcement',
      content: 'Updated body',
      status: 'PUBLISHED',
      publishTime: '2026-03-29T10:00:00.000Z',
      expireTime: null,
      expiredTime: null,
      createdBy: '11111111-1111-4111-8111-111111111111',
      updatedBy: '11111111-1111-4111-8111-111111111111',
      createdTime: '2026-03-29T09:00:00.000Z',
      updatedTime: '2026-03-30T09:00:00.000Z',
    },
  };

  const expectUpdatedAnnouncementResponse = (response: {
    statusCode: number;
    json: () => unknown;
  }) => {
    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual(updatedAnnouncementResponse);
  };

  it('rejects overlapping effective announcement windows', async () => {
    app = await createAnnouncementTestApp(['announcement.manage']);

    mockOverlapSelect([
      {
        id: 'announcement-existing',
        publishTime: new Date('2026-05-01T10:00:00.000Z'),
        expireTime: new Date('2026-05-03T10:00:00.000Z'),
      },
    ]);

    const response = await getApp().inject({
      method: 'POST',
      url: '/api/management/announcements',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'admin-token',
      },
      payload: {
        title: 'New announcement',
        content: 'Content',
        status: 'SCHEDULED',
        publish_time: '2026-05-02T09:00:00.000Z',
        expire_time: '2026-05-05T09:00:00.000Z',
      },
    });

    expect(response.statusCode).toBe(409);
    expect(response.json()).toEqual({
      ok: false,
      error: {
        code: 'ANNOUNCEMENT_WINDOW_CONFLICT',
        message: 'The announcement window overlaps with announcement announcement-existing.',
      },
    });
  });

  it('updates a published announcement successfully when its own window is unchanged', async () => {
    app = await createAnnouncementTestApp(['announcement.manage']);

    mockStatusAndOverlapSelect('PUBLISHED', []);
    const getCapturedValues = mockAnnouncementUpdate({
      id: '11111111-1111-4111-8111-222222222222',
      title: 'Current announcement',
      content: 'Updated body',
      status: 'PUBLISHED',
      publishTime: new Date('2026-03-29T10:00:00.000Z'),
      expireTime: null,
      expiredTime: null,
      createdBy: '11111111-1111-4111-8111-111111111111',
      updatedBy: '11111111-1111-4111-8111-111111111111',
      createdTime: new Date('2026-03-29T09:00:00.000Z'),
      updatedTime: new Date('2026-03-30T09:00:00.000Z'),
    });

    const response = await getApp().inject({
      method: 'POST',
      url: '/api/management/announcements',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'admin-token',
      },
      payload: {
        id: '11111111-1111-4111-8111-222222222222',
        title: 'Current announcement',
        content: 'Updated body',
        status: 'PUBLISHED',
        publish_time: '2026-03-29T10:00:00.000Z',
        expire_time: null,
      },
    });

    expect(getCapturedValues()).toMatchObject({
      title: 'Current announcement',
      content: 'Updated body',
      status: 'PUBLISHED',
      updated_by: '11111111-1111-4111-8111-111111111111',
    });
    expectUpdatedAnnouncementResponse(response);
  });

  it('rejects downgrading non-draft announcements to draft', async () => {
    app = await createAnnouncementTestApp(['announcement.manage']);

    mockStatusAndOverlapSelect('PUBLISHED', []);

    const response = await getApp().inject({
      method: 'POST',
      url: '/api/management/announcements',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'admin-token',
      },
      payload: {
        id: '11111111-1111-4111-8111-222222222222',
        title: 'Current announcement',
        content: 'Body',
        status: 'DRAFT',
        publish_time: null,
        expire_time: null,
      },
    });

    expect(response.statusCode).toBe(409);
    expect(response.json()).toEqual({
      ok: false,
      error: {
        code: 'ANNOUNCEMENT_DRAFT_FORBIDDEN',
        message: 'Published or scheduled announcements cannot be reverted to draft.',
      },
    });
  });

  it('auto-fills publish_time for immediate publish when omitted', async () => {
    app = await createAnnouncementTestApp(['announcement.manage']);

    mockOverlapSelect([]);
    const getCapturedValues = mockAnnouncementInsert((values) => ({
      id: 'announcement-created',
      title: 'Immediate announcement',
      content: 'Immediate body',
      status: 'PUBLISHED',
      publishTime: values.publish_time,
      expireTime: null,
      expiredTime: null,
      createdBy: baseAdminUser.id,
      updatedBy: baseAdminUser.id,
      createdTime: new Date('2026-03-30T09:00:00.000Z'),
      updatedTime: new Date('2026-03-30T09:00:00.000Z'),
    }));

    const response = await getApp().inject({
      method: 'POST',
      url: '/api/management/announcements',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'admin-token',
      },
      payload: {
        title: 'Immediate announcement',
        content: 'Immediate body',
        status: 'PUBLISHED',
        publish_time: null,
        expire_time: null,
      },
    });

    expect(response.statusCode).toBe(200);
    expect(getCapturedValues()?.status).toBe('PUBLISHED');
    expect(getCapturedValues()?.publish_time).toBeInstanceOf(Date);
  });

  it('archives an active published announcement immediately', async () => {
    app = await createAnnouncementTestApp(['announcement.manage']);

    mockAnnouncementByIdSelect({
      id: '11111111-1111-4111-8111-222222222222',
      title: 'Current announcement',
      content: 'Body',
      status: 'PUBLISHED',
      publishTime: new Date(Date.now() - 60_000),
      expireTime: null,
      expiredTime: null,
      createdBy: baseAdminUser.id,
      updatedBy: baseAdminUser.id,
      createdTime: new Date('2026-03-29T09:00:00.000Z'),
      updatedTime: new Date('2026-03-29T10:00:00.000Z'),
    });

    const getCapturedValues = mockAnnouncementUpdate({
      id: '11111111-1111-4111-8111-222222222222',
      title: 'Current announcement',
      content: 'Body',
      status: 'EXPIRED',
      publishTime: new Date(Date.now() - 60_000),
      expireTime: new Date(),
      expiredTime: new Date(),
      createdBy: baseAdminUser.id,
      updatedBy: baseAdminUser.id,
      createdTime: new Date('2026-03-29T09:00:00.000Z'),
      updatedTime: new Date('2026-03-30T09:00:00.000Z'),
    });

    const response = await getApp().inject({
      method: 'POST',
      url: '/api/management/announcements/11111111-1111-4111-8111-222222222222/archive',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'admin-token',
      },
    });

    expect(response.statusCode).toBe(200);
    expect(getCapturedValues()).toMatchObject({
      status: 'EXPIRED',
      updated_by: baseAdminUser.id,
    });
    expect(getCapturedValues()?.expire_time).toBeInstanceOf(Date);
    expect(getCapturedValues()?.expired_time).toBeInstanceOf(Date);
  });

  it('only allows deleting draft announcements', async () => {
    app = await createAnnouncementTestApp(['announcement.manage']);

    mockAnnouncementDeleteSelect('PUBLISHED');

    const response = await getApp().inject({
      method: 'POST',
      url: '/api/management/announcements/11111111-1111-4111-8111-222222222222/delete',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'admin-token',
      },
    });

    expect(response.statusCode).toBe(409);
    expect(response.json()).toEqual({
      ok: false,
      error: {
        code: 'ANNOUNCEMENT_DELETE_FORBIDDEN',
        message: 'Only draft announcements can be deleted.',
      },
    });
  });
});

import { Announcements } from '@zhblogs/db';

import { afterEach, describe, expect, it, vi } from 'vitest';

import type { AuthUser } from '@/domain/auth/types/auth.types';

import { TEST_AUTH_COOKIES } from '../config';
import { createTestApp } from '../create-test-app';

const baseAdminUser: AuthUser = {
  id: '11111111-1111-4111-8111-111111111111',
  email: 'admin@example.com',
  nickname: 'Admin',
  avatarUrl: null,
  role: 'ADMIN',
  permissions: [],
  isActive: true,
  isVerified: true,
  hasPassword: true,
  hasGithub: true,
  authVersion: 1,
  adminGrantedBy: 'sys-admin-id',
  adminGrantedTime: '2026-03-19T00:00:00.000Z',
};

describe('announcement management routes', () => {
  let app: ReturnType<typeof createTestApp> | undefined;

  afterEach(async () => {
    vi.restoreAllMocks();
    await app?.close();
    app = undefined;
  });

  it('protects announcement routes from admins without announcement.manage', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.auth.getCurrentUser = vi.fn(async () => baseAdminUser);

    const response = await app.inject({
      method: 'GET',
      url: '/api/management/announcements',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'admin-token',
      },
    });

    expect(response.statusCode).toBe(403);
  });

  it('returns paginated management announcement list', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.auth.getCurrentUser = vi.fn(async () => ({
      ...baseAdminUser,
      permissions: ['announcement.manage'],
    }));

    app.db.read.select = vi.fn((fields?: Record<string, unknown>) => {
      if (fields && 'total' in fields) {
        return {
          from(table: unknown) {
            expect(table).toBe(Announcements);

            return Promise.resolve([
              {
                total: 12,
              },
            ]);
          },
        };
      }

      return {
        from(table: unknown) {
          expect(table).toBe(Announcements);

          return {
            orderBy: vi.fn(() => ({
              limit: vi.fn((limitValue: number) => {
                expect(limitValue).toBe(5);

                return {
                  offset: vi.fn(async (offsetValue: number) => {
                    expect(offsetValue).toBe(5);

                    return [
                      {
                        id: 'announcement-2',
                        title: 'Announcement 2',
                        content: 'Body',
                        status: 'PUBLISHED',
                        publishTime: new Date('2026-03-29T10:00:00.000Z'),
                        expireTime: null,
                        expiredTime: null,
                        createdBy: baseAdminUser.id,
                        updatedBy: baseAdminUser.id,
                        createdTime: new Date('2026-03-29T09:00:00.000Z'),
                        updatedTime: new Date('2026-03-29T11:00:00.000Z'),
                      },
                    ];
                  }),
                };
              }),
            })),
          };
        },
      };
    }) as unknown as typeof app.db.read.select;

    const response = await app.inject({
      method: 'GET',
      url: '/api/management/announcements?page=2&pageSize=5',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'admin-token',
      },
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({
      ok: true,
      data: {
        items: [
          {
            id: 'announcement-2',
            title: 'Announcement 2',
            content: 'Body',
            status: 'PUBLISHED',
            publishTime: '2026-03-29T10:00:00.000Z',
            expireTime: null,
            expiredTime: null,
            createdBy: baseAdminUser.id,
            updatedBy: baseAdminUser.id,
            createdTime: '2026-03-29T09:00:00.000Z',
            updatedTime: '2026-03-29T11:00:00.000Z',
          },
        ],
        pagination: {
          page: 2,
          pageSize: 5,
          totalItems: 12,
          totalPages: 3,
        },
      },
    });
  });

  it('rejects overlapping effective announcement windows', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.auth.getCurrentUser = vi.fn(async () => ({
      ...baseAdminUser,
      permissions: ['announcement.manage'],
    }));
    app.db.read.select = vi.fn(() => ({
      from(table: unknown) {
        expect(table).toBe(Announcements);

        return {
          where: vi.fn(async () => [
            {
              id: 'announcement-existing',
              publishTime: new Date('2026-05-01T10:00:00.000Z'),
              expireTime: new Date('2026-05-03T10:00:00.000Z'),
            },
          ]),
        };
      },
    })) as unknown as typeof app.db.read.select;

    const response = await app.inject({
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
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.auth.getCurrentUser = vi.fn(async () => ({
      ...baseAdminUser,
      permissions: ['announcement.manage'],
    }));

    app.db.read.select = vi.fn((fields?: Record<string, unknown>) => {
      if (fields && 'status' in fields) {
        return {
          from(table: unknown) {
            expect(table).toBe(Announcements);

            return {
              where: vi.fn(() => ({
                limit: vi.fn(async () => [
                  {
                    id: '11111111-1111-4111-8111-222222222222',
                    status: 'PUBLISHED',
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
            where: vi.fn(async () => []),
          };
        },
      };
    }) as unknown as typeof app.db.read.select;

    let capturedValues: Record<string, unknown> | undefined;

    app.db.write.update = vi.fn((table: unknown) => {
      expect(table).toBe(Announcements);

      return {
        set: vi.fn((values: Record<string, unknown>) => {
          capturedValues = values;

          return {
            where: vi.fn(() => ({
              returning: vi.fn(async () => [
                {
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
                },
              ]),
            })),
          };
        }),
      };
    }) as unknown as typeof app.db.write.update;

    const response = await app.inject({
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

    expect(response.statusCode).toBe(200);
    expect(capturedValues).toMatchObject({
      title: 'Current announcement',
      content: 'Updated body',
      status: 'PUBLISHED',
      updated_by: '11111111-1111-4111-8111-111111111111',
    });
    expect(response.json()).toEqual({
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
    });
  });

  it('auto-fills publish_time for immediate publish when omitted', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.auth.getCurrentUser = vi.fn(async () => ({
      ...baseAdminUser,
      permissions: ['announcement.manage'],
    }));

    app.db.read.select = vi.fn(() => ({
      from(table: unknown) {
        expect(table).toBe(Announcements);

        return {
          where: vi.fn(async () => []),
        };
      },
    })) as unknown as typeof app.db.read.select;

    let capturedValues: Record<string, unknown> | undefined;

    app.db.write.insert = vi.fn((table: unknown) => {
      expect(table).toBe(Announcements);

      return {
        values: vi.fn((values: Record<string, unknown>) => {
          capturedValues = values;

          return {
            returning: vi.fn(async () => [
              {
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
              },
            ]),
          };
        }),
      };
    }) as unknown as typeof app.db.write.insert;

    const response = await app.inject({
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
    expect(capturedValues?.status).toBe('PUBLISHED');
    expect(capturedValues?.publish_time).toBeInstanceOf(Date);
  });

  it('archives an active published announcement immediately', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.auth.getCurrentUser = vi.fn(async () => ({
      ...baseAdminUser,
      permissions: ['announcement.manage'],
    }));

    app.db.read.select = vi.fn(() => ({
      from(table: unknown) {
        expect(table).toBe(Announcements);

        return {
          where: vi.fn(() => ({
            limit: vi.fn(async () => [
              {
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
              },
            ]),
          })),
        };
      },
    })) as unknown as typeof app.db.read.select;

    let capturedValues: Record<string, unknown> | undefined;

    app.db.write.update = vi.fn((table: unknown) => {
      expect(table).toBe(Announcements);

      return {
        set: vi.fn((values: Record<string, unknown>) => {
          capturedValues = values;

          return {
            where: vi.fn(() => ({
              returning: vi.fn(async () => [
                {
                  id: '11111111-1111-4111-8111-222222222222',
                  title: 'Current announcement',
                  content: 'Body',
                  status: 'EXPIRED',
                  publishTime: new Date(Date.now() - 60_000),
                  expireTime: values.expire_time,
                  expiredTime: values.expired_time,
                  createdBy: baseAdminUser.id,
                  updatedBy: baseAdminUser.id,
                  createdTime: new Date('2026-03-29T09:00:00.000Z'),
                  updatedTime: new Date('2026-03-30T09:00:00.000Z'),
                },
              ]),
            })),
          };
        }),
      };
    }) as unknown as typeof app.db.write.update;

    const response = await app.inject({
      method: 'POST',
      url: '/api/management/announcements/11111111-1111-4111-8111-222222222222/archive',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'admin-token',
      },
    });

    expect(response.statusCode).toBe(200);
    expect(capturedValues).toMatchObject({
      status: 'EXPIRED',
      updated_by: baseAdminUser.id,
    });
    expect(capturedValues?.expire_time).toBeInstanceOf(Date);
    expect(capturedValues?.expired_time).toBeInstanceOf(Date);
  });

  it('only allows deleting draft announcements', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.auth.getCurrentUser = vi.fn(async () => ({
      ...baseAdminUser,
      permissions: ['announcement.manage'],
    }));
    app.db.read.select = vi.fn(() => ({
      from(table: unknown) {
        expect(table).toBe(Announcements);

        return {
          where: vi.fn(() => ({
            limit: vi.fn(async () => [
              {
                id: '11111111-1111-4111-8111-222222222222',
                status: 'PUBLISHED',
              },
            ]),
          })),
        };
      },
    })) as unknown as typeof app.db.read.select;

    const response = await app.inject({
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

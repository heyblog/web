import { afterEach, describe, expect, it, vi } from 'vitest';

import { POST as permissionsPost } from '@/pages/management/users/[userId]/permissions';
import { POST as rolePost } from '@/pages/management/users/[userId]/role';

const managedUserFixture = {
  id: '11111111-1111-4111-8111-111111111111',
  email: 'alice@example.com',
  nickname: 'Alice',
  avatarUrl: null,
  role: 'ADMIN',
  permissions: ['user.manage'],
  isActive: true,
  isVerified: true,
  hasPassword: true,
  hasGithub: false,
  authVersion: 1,
  adminGrantedBy: '22222222-2222-4222-8222-222222222222',
  adminGrantedTime: '2026-04-09T10:00:00.000Z',
  createdTime: '2026-01-01T10:00:00.000Z',
  lastLoginTime: '2026-04-09T10:00:00.000Z',
};

const createRoleContext = (request: Request, userId = managedUserFixture.id) =>
  ({
    request,
    params: {
      userId,
    },
  }) as unknown as Parameters<typeof rolePost>[0];

const createPermissionsContext = (request: Request, userId = managedUserFixture.id) =>
  ({
    request,
    params: {
      userId,
    },
  }) as unknown as Parameters<typeof permissionsPost>[0];

const createRoleRequest = (
  intent: string,
  accept = 'application/json',
  extraHeaders: Record<string, string> = {},
): Request => {
  const body = new FormData();
  body.set('intent', intent);

  return new Request(`http://127.0.0.1:9902/management/users/${managedUserFixture.id}/role`, {
    method: 'POST',
    headers: {
      accept,
      ...extraHeaders,
    },
    body,
  });
};

const createPermissionsRequest = (
  permissions: string[],
  accept = 'application/json',
  extraHeaders: Record<string, string> = {},
): Request => {
  const body = new FormData();

  for (const permission of permissions) {
    body.append('permissions', permission);
  }

  return new Request(
    `http://127.0.0.1:9902/management/users/${managedUserFixture.id}/permissions`,
    {
      method: 'POST',
      headers: {
        accept,
        ...extraHeaders,
      },
      body,
    },
  );
};

describe('management user authorization routes', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns JSON success payload for role updates', async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            ok: true,
            data: managedUserFixture,
          }),
        ),
    );
    vi.stubGlobal('fetch', fetchMock);

    const response = await rolePost(createRoleContext(createRoleRequest('grant-admin')));

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:9901/api/management/users/11111111-1111-4111-8111-111111111111/grant-admin',
      expect.objectContaining({
        method: 'POST',
      }),
    );
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({
      ok: true,
      status: 'role_granted',
      target: managedUserFixture.id,
      redirect: null,
      message: '',
      data: managedUserFixture,
    });
  });

  it('returns JSON even when the request prefers html', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              ok: true,
              data: managedUserFixture,
            }),
          ),
      ),
    );

    const response = await rolePost(
      createRoleContext(createRoleRequest('revoke-admin', 'text/html')),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get('content-type')).toContain('application/json');
    expect(await response.json()).toEqual({
      ok: true,
      status: 'role_revoked',
      target: managedUserFixture.id,
      redirect: null,
      message: '',
      data: managedUserFixture,
    });
  });

  it('returns JSON unauthorized payload for role updates', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              error: {
                message: 'expired',
              },
            }),
            { status: 401 },
          ),
      ),
    );

    const response = await rolePost(createRoleContext(createRoleRequest('grant-admin')));

    expect(response.status).toBe(401);
    expect(await response.json()).toEqual({
      ok: false,
      code: 'unauthorized',
      target: managedUserFixture.id,
      redirect: '/login?next=%2Fmanagement%2Fusers',
      message: '登录状态已过期，请重新登录。',
      data: null,
    });
  });

  it('returns JSON error payload for forbidden role updates', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              error: {
                message: '  Permission denied for this action.  ',
              },
            }),
            { status: 403 },
          ),
      ),
    );

    const response = await rolePost(createRoleContext(createRoleRequest('revoke-admin')));

    expect(response.status).toBe(403);
    expect(await response.json()).toEqual({
      ok: false,
      code: 'role_update_failed',
      target: managedUserFixture.id,
      redirect: null,
      message: 'Permission denied for this action.',
      data: null,
    });
  });

  it('returns JSON success payload for permission updates and de-duplicates permissions', async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            ok: true,
            data: {
              ...managedUserFixture,
              permissions: ['feedback.review', 'user.manage'],
            },
          }),
        ),
    );
    vi.stubGlobal('fetch', fetchMock);

    const response = await permissionsPost(
      createPermissionsContext(
        createPermissionsRequest(['user.manage', 'feedback.review', 'user.manage']),
      ),
    );

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:9901/api/management/users/11111111-1111-4111-8111-111111111111/permissions',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({
          permissions: ['user.manage', 'feedback.review'],
        }),
      }),
    );
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({
      ok: true,
      status: 'permissions_updated',
      target: managedUserFixture.id,
      redirect: null,
      message: '',
      data: {
        ...managedUserFixture,
        permissions: ['feedback.review', 'user.manage'],
      },
    });
  });

  it('returns JSON conflict payload for permission updates', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              error: {
                message: '  Authorization state changed.  ',
              },
            }),
            { status: 409 },
          ),
      ),
    );

    const response = await permissionsPost(
      createPermissionsContext(createPermissionsRequest(['user.manage'])),
    );

    expect(response.status).toBe(409);
    expect(await response.json()).toEqual({
      ok: false,
      code: 'permission_update_failed',
      target: managedUserFixture.id,
      redirect: null,
      message: 'Authorization state changed.',
      data: null,
    });
  });

  it('rejects invalid role requests with a 400 response', async () => {
    const response = await rolePost(createRoleContext(createRoleRequest('invalid-intent')));

    expect(response.status).toBe(400);
    expect(await response.json()).toEqual({
      ok: false,
      code: 'invalid_role_request',
      target: managedUserFixture.id,
      redirect: null,
      message: '角色授权请求无效，请刷新后重试。',
      data: null,
    });
  });

  it('rejects invalid permission requests with a 400 response', async () => {
    const response = await permissionsPost(
      createPermissionsContext(createPermissionsRequest(['not.a.valid.permission'])),
    );

    expect(response.status).toBe(400);
    expect(await response.json()).toEqual({
      ok: false,
      code: 'invalid_permissions_request',
      target: managedUserFixture.id,
      redirect: null,
      message: '模块权限请求无效，请刷新后重试。',
      data: null,
    });
  });

  it('rejects malformed managed user ids before proxying the request', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);

    const response = await permissionsPost(
      createPermissionsContext(createPermissionsRequest(['user.manage']), 'not-a-uuid'),
    );

    expect(fetchMock).not.toHaveBeenCalled();
    expect(response.status).toBe(400);
  });
});

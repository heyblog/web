import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  submitPermissionAuthorizationAction,
  submitRoleAuthorizationAction,
} from '@/application/management/user-authorization.browser';

describe('user authorization browser actions', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('parses successful role updates', async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            ok: true,
            status: 'role_granted',
            target: 'user-1',
            data: {
              id: 'user-1',
              role: 'ADMIN',
            },
          }),
          {
            status: 200,
          },
        ),
    );
    vi.stubGlobal('fetch', fetchMock);

    const result = await submitRoleAuthorizationAction('user-1', 'grant-admin');

    expect(result.ok).toBe(true);
    expect(result.status).toBe('role_granted');
    expect(result.target).toBe('user-1');
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('parses failed permission updates with redirect target', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              ok: false,
              code: 'unauthorized',
              message: '登录状态已过期，请重新登录。',
              redirect: '/login?next=%2Fmanagement%2Fusers',
            }),
            {
              status: 401,
            },
          ),
      ),
    );

    const result = await submitPermissionAuthorizationAction('user-1', ['user.manage']);

    expect(result.ok).toBe(false);
    expect(result.code).toBe('unauthorized');
    expect(result.redirect).toBe('/login?next=%2Fmanagement%2Fusers');
    expect(result.message).toBe('登录状态已过期，请重新登录。');
  });
});

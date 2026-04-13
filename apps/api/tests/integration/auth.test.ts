import { afterEach, describe, expect, it, vi } from 'vitest';

import type { AuthUser, ManagedUserSnapshot } from '@/domain/auth/types/auth.types';

import { TEST_AUTH_COOKIES } from '../config';
import { createTestApp } from '../create-test-app';

const userFixture: AuthUser = {
  id: '11111111-1111-4111-8111-111111111111',
  email: 'user@example.com',
  nickname: 'User',
  avatarUrl: null,
  role: 'USER',
  permissions: [],
  isActive: true,
  isVerified: true,
  hasPassword: true,
  hasGithub: false,
  authVersion: 1,
  adminGrantedBy: null,
  adminGrantedTime: null,
};

describe('auth routes', () => {
  let app: ReturnType<typeof createTestApp> | undefined;

  afterEach(async () => {
    vi.restoreAllMocks();
    await app?.close();
    app = undefined;
  });

  it('returns 401 for /auth/me without a session cookie', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    const response = await app.inject({
      method: 'GET',
      url: '/auth/me',
    });

    expect(response.statusCode).toBe(401);
  });

  it('returns current user when auth service resolves session', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.auth.getCurrentUser = vi.fn(async () => userFixture);

    const response = await app.inject({
      method: 'GET',
      url: '/auth/me',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'test-token',
      },
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({
      ok: true,
      user: userFixture,
    });
  });

  it('uses auth service for refresh and logout endpoints', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.auth.refreshSession = vi.fn(async (_request, reply) => {
      reply.header('set-cookie', `${TEST_AUTH_COOKIES.access}=refreshed`);
      return userFixture;
    });
    app.auth.logout = vi.fn(async (_request, reply) => {
      reply.header('set-cookie', `${TEST_AUTH_COOKIES.access}=; Max-Age=0`);
    });

    const refreshResponse = await app.inject({
      method: 'POST',
      url: '/auth/refresh',
      cookies: {
        [TEST_AUTH_COOKIES.refresh]: 'refresh-token',
      },
    });
    const logoutResponse = await app.inject({
      method: 'POST',
      url: '/auth/logout',
    });

    expect(refreshResponse.statusCode).toBe(200);
    expect(refreshResponse.json()).toEqual({
      ok: true,
      user: userFixture,
    });
    expect(logoutResponse.statusCode).toBe(200);
    expect(logoutResponse.json()).toEqual({
      ok: true,
    });
  });

  it('protects user-management routes from admins without user.manage', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    const adminUser: AuthUser = {
      ...userFixture,
      role: 'ADMIN',
    };

    app.auth.getCurrentUser = vi.fn(async () => adminUser);

    const response = await app.inject({
      method: 'GET',
      url: '/api/management/users',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'test-token',
      },
    });

    expect(response.statusCode).toBe(403);
  });

  it('allows delegated user managers to access role management routes', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    const delegatedManager: AuthUser = {
      ...userFixture,
      role: 'ADMIN',
      permissions: ['user.manage'],
    };
    const managedUser: ManagedUserSnapshot = {
      ...delegatedManager,
      createdTime: '2026-03-19T00:00:00.000Z',
      lastLoginTime: '2026-03-19T00:00:00.000Z',
    };
    const managedUsers: ManagedUserSnapshot[] = [managedUser];

    app.auth.getCurrentUser = vi.fn(async () => delegatedManager);
    app.auth.listManagedUsers = vi.fn(async () => managedUsers);
    app.auth.grantAdminRole = vi.fn(async () => managedUser);
    const revokedUser: ManagedUserSnapshot = {
      ...managedUser,
      role: 'USER',
      permissions: [],
    };

    app.auth.revokeAdminRole = vi.fn(async () => revokedUser);

    const listResponse = await app.inject({
      method: 'GET',
      url: '/api/management/users',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'admin-token',
      },
    });
    const grantResponse = await app.inject({
      method: 'POST',
      url: `/api/management/users/${userFixture.id}/grant-admin`,
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'admin-token',
      },
    });
    const revokeResponse = await app.inject({
      method: 'POST',
      url: `/api/management/users/${userFixture.id}/revoke-admin`,
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'admin-token',
      },
    });

    expect(listResponse.statusCode).toBe(200);
    expect(listResponse.json()).toEqual({
      ok: true,
      data: managedUsers,
    });
    expect(grantResponse.statusCode).toBe(200);
    expect(revokeResponse.statusCode).toBe(200);
  });

  it('redirects github auth start to oauth flow', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    const response = await app.inject({
      method: 'GET',
      url: '/auth/github?next=%2Fmanagement',
    });

    expect(response.statusCode).toBe(302);
    expect(response.headers.location).toBe('/auth/github/start');
    expect(response.cookies.find((cookie) => cookie.name === 'zhblogs_auth_return_to')?.value).toBe(
      '/management',
    );
  });

  it('uses auth service for local auth endpoints', async () => {
    app = createTestApp({
      disableExternalServices: true,
    });

    await app.ready();

    app.auth.completeGithubLogin = vi.fn(async (_request, reply) => {
      reply.redirect('http://127.0.0.1:9101/dashboard');
    });
    app.auth.loginWithPassword = vi.fn(async (_identifier, _password, reply) => {
      reply.header('set-cookie', `${TEST_AUTH_COOKIES.access}=login-token`);
      return userFixture;
    });
    app.auth.registerLocalAccount = vi.fn(async () => undefined);
    app.auth.verifyEmailToken = vi.fn(async () => undefined);
    app.auth.resendVerificationEmail = vi.fn(async () => undefined);
    app.auth.startPasswordReset = vi.fn(async () => undefined);
    app.auth.resetPassword = vi.fn(async () => undefined);
    app.auth.setPassword = vi.fn(async (_actor, _payload, reply) => {
      reply.header('set-cookie', `${TEST_AUTH_COOKIES.access}=password-token`);
      return userFixture;
    });
    app.auth.unbindGithub = vi.fn(async (_actor, reply) => {
      reply.header('set-cookie', `${TEST_AUTH_COOKIES.access}=github-unbound-token`);
      return {
        ...userFixture,
        hasGithub: false,
      };
    });

    app.auth.getCurrentUser = vi.fn(async () => userFixture);

    const loginResponse = await app.inject({
      method: 'POST',
      url: '/auth/login',
      payload: {
        identifier: 'user@example.com',
        password: 'hunter2-pass',
      },
    });
    const githubExchangeResponse = await app.inject({
      method: 'GET',
      url: '/auth/github/exchange?code=test-code&state=test-state',
    });
    const registerResponse = await app.inject({
      method: 'POST',
      url: '/auth/register',
      payload: {
        username: 'user_name',
        email: 'user@example.com',
        password: 'hunter2-pass',
      },
    });
    const verifyResponse = await app.inject({
      method: 'POST',
      url: '/auth/verify-email',
      payload: {
        token: 'verify-token',
      },
    });
    const resendResponse = await app.inject({
      method: 'POST',
      url: '/auth/verify-email/resend',
      payload: {
        email: 'user@example.com',
      },
    });
    const forgotResponse = await app.inject({
      method: 'POST',
      url: '/auth/password/forgot',
      payload: {
        email: 'user@example.com',
      },
    });
    const resetResponse = await app.inject({
      method: 'POST',
      url: '/auth/password/reset',
      payload: {
        token: 'reset-token',
        password: 'new-password-123',
      },
    });
    const passwordResponse = await app.inject({
      method: 'POST',
      url: '/auth/password',
      payload: {
        currentPassword: 'old-password',
        nextPassword: 'new-password-123',
      },
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'test-token',
      },
    });
    const unbindGithubResponse = await app.inject({
      method: 'POST',
      url: '/auth/github/unbind',
      cookies: {
        [TEST_AUTH_COOKIES.access]: 'test-token',
      },
    });

    expect(loginResponse.statusCode).toBe(200);
    expect(loginResponse.json()).toEqual({
      ok: true,
      user: userFixture,
    });
    expect(githubExchangeResponse.statusCode).toBe(302);
    expect(githubExchangeResponse.headers.location).toBe('http://127.0.0.1:9101/dashboard');
    expect(registerResponse.statusCode).toBe(200);
    expect(registerResponse.json()).toEqual({ ok: true });
    expect(verifyResponse.statusCode).toBe(200);
    expect(resendResponse.statusCode).toBe(200);
    expect(forgotResponse.statusCode).toBe(200);
    expect(resetResponse.statusCode).toBe(200);
    expect(passwordResponse.statusCode).toBe(200);
    expect(passwordResponse.json()).toEqual({
      ok: true,
      user: userFixture,
    });
    expect(unbindGithubResponse.statusCode).toBe(200);
    expect(unbindGithubResponse.json()).toEqual({
      ok: true,
      user: {
        ...userFixture,
        hasGithub: false,
      },
    });
  });

  it('fails startup when required auth config is missing', async () => {
    app = createTestApp({
      disableExternalServices: true,
      envOverrides: {
        API_GITHUB_CLIENT_ID: '',
      },
    });

    await expect(app.ready()).rejects.toThrow(/API_GITHUB_CLIENT_ID/);
  });
});

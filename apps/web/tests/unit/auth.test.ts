import { describe, expect, it, vi } from 'vitest';

import {
  getProtectionLevel,
  hasAccessToPath,
  type SessionUser,
} from '@/application/auth/auth.guard';
import {
  forwardSetCookieHeaders,
  getGithubBindHref,
  getGithubUnbindHref,
  getLoginHref,
  readSessionUser,
} from '@/application/auth/auth.server';
import { buildRedirectUrl, resolvePostLoginRedirect } from '@/application/auth/auth-route.server';
import { getWebBaseUrl } from '@tests/setup/env';

import { apiStubs } from '../setup/api-stubs';
import { adminUserFixture } from '../setup/fixtures';

const adminUser: SessionUser = adminUserFixture;

describe('route protection helpers', () => {
  it('detects protected route levels', () => {
    expect(getProtectionLevel('/')).toBeNull();
    expect(getProtectionLevel('/dashboard')).toBe('authenticated');
    expect(getProtectionLevel('/management')).toBe('management-home');
    expect(getProtectionLevel('/management/users')).toBe('user.manage');
    expect(getProtectionLevel('/management/site-submissions')).toBe('site_audit.review');
    expect(getProtectionLevel('/management/announcements')).toBe('announcement.manage');
  });

  it('checks access against route level', () => {
    expect(hasAccessToPath(null, '/dashboard')).toBe(false);
    expect(hasAccessToPath(adminUser, '/dashboard')).toBe(true);
    expect(hasAccessToPath(adminUser, '/management')).toBe(true);
    expect(hasAccessToPath(adminUser, '/management/users')).toBe(false);
    expect(hasAccessToPath(adminUser, '/management/site-submissions')).toBe(false);
    expect(hasAccessToPath(adminUser, '/management/announcements')).toBe(false);
    expect(
      hasAccessToPath(
        {
          ...adminUser,
          permissions: ['site_audit.review'],
        },
        '/management/site-submissions',
      ),
    ).toBe(true);
    expect(
      hasAccessToPath(
        {
          ...adminUser,
          permissions: ['announcement.manage'],
        },
        '/management/announcements',
      ),
    ).toBe(true);
    expect(
      hasAccessToPath(
        {
          ...adminUser,
          role: 'SYS_ADMIN',
        },
        '/management/users',
      ),
    ).toBe(true);
  });
});

describe('server auth helpers', () => {
  it('builds github auth links against the API base url', () => {
    expect(getLoginHref('/management')).toBe('/auth/github?next=%2Fmanagement');
    expect(getGithubBindHref()).toBe('/auth/github/bind');
    expect(getGithubUnbindHref()).toBe('/auth/github/unbind');
  });

  it('builds absolute redirect urls for response redirects', () => {
    const request = new Request(`${getWebBaseUrl()}/forgot-password`);

    expect(
      buildRedirectUrl(request, '/login', {
        status: 'reset-sent',
        next: '/management',
      }),
    ).toBe(`${getWebBaseUrl()}/login?status=reset-sent&next=%2Fmanagement`);
  });

  it('redirects users away from management paths after login', () => {
    expect(resolvePostLoginRedirect('/management', { role: 'USER', permissions: [] })).toBe(
      '/dashboard',
    );
    expect(resolvePostLoginRedirect('/management/users', { role: 'USER', permissions: [] })).toBe(
      '/dashboard',
    );
    expect(resolvePostLoginRedirect('/dashboard', { role: 'USER', permissions: [] })).toBe(
      '/dashboard',
    );
    expect(resolvePostLoginRedirect('/dashboard/account', { role: 'USER', permissions: [] })).toBe(
      '/dashboard/account',
    );
    expect(resolvePostLoginRedirect(null, { role: 'USER', permissions: [] })).toBe('/dashboard');
    expect(resolvePostLoginRedirect('/management', { role: 'ADMIN', permissions: [] })).toBe(
      '/dashboard',
    );
    expect(
      resolvePostLoginRedirect('/management', {
        role: 'ADMIN',
        permissions: ['site_audit.review'],
      }),
    ).toBe('/dashboard');
    expect(resolvePostLoginRedirect(null, { role: 'ADMIN', permissions: [] })).toBe('/dashboard');
  });

  it('returns null when auth API says session is missing', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => apiStubs.auth.sessionMissing()),
    );

    const result = await readSessionUser(
      new Request('http://127.0.0.1:9101/dashboard', {
        headers: {
          cookie: 'zhblogs_access_token=test',
        },
      }),
    );

    expect(result).toBeNull();
  });

  it('returns current user when auth API succeeds', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => apiStubs.auth.sessionOk(adminUser)),
    );

    const result = await readSessionUser(
      new Request(`${getWebBaseUrl()}/dashboard`, {
        headers: {
          cookie: 'zhblogs_access_token=test',
        },
      }),
    );

    expect(result).toEqual(adminUser);
  });

  it('forwards set-cookie headers', () => {
    const source = new Headers();
    const target = new Headers();

    const fakeHeaders = Object.assign(source, {
      getSetCookie: () => ['foo=bar; Path=/', 'baz=qux; Path=/'],
    }) as Headers & { getSetCookie: () => string[] };

    forwardSetCookieHeaders(fakeHeaders, target);

    expect(target.get('set-cookie')).toContain('foo=bar');
  });
});

import type { SessionUser } from './auth.guard';

const DEFAULT_API_BASE_URL = 'http://127.0.0.1:9201';

const trimTrailingSlash = (value: string): string => value.replace(/\/+$/, '');
const buildWebAuthPath = (pathname: string, params: Record<string, string> = {}): string => {
  const target = new URL(pathname, 'http://zhblogs.local');

  for (const [key, value] of Object.entries(params)) {
    if (value) {
      target.searchParams.set(key, value);
    }
  }

  return `${target.pathname}${target.search}`;
};

export const getApiBaseUrl = (): string =>
  trimTrailingSlash(import.meta.env.PUBLIC_API_BASE_URL || DEFAULT_API_BASE_URL);

export const getLoginHref = (next?: string): string => {
  const normalizedNext = next?.trim();
  return buildWebAuthPath('/auth/github', normalizedNext ? { next: normalizedNext } : {});
};

export const getGithubBindHref = (): string => '/auth/github/bind';
export const getGithubUnbindHref = (): string => '/auth/github/unbind';

export const readSessionUser = async (request: Request): Promise<SessionUser | null> => {
  const cookie = request.headers.get('cookie');

  if (!cookie) {
    return null;
  }

  const response = await fetch(`${getApiBaseUrl()}/auth/me`, {
    headers: {
      cookie,
      accept: 'application/json',
    },
  });

  if (response.status === 401) {
    return null;
  }

  if (!response.ok) {
    throw new Error('failed to resolve current session user');
  }

  const payload = (await response.json()) as {
    ok: boolean;
    user: SessionUser;
  };

  return payload.user;
};

export const fetchProtectedJson = async <T>(
  request: Request,
  path: string,
  init?: RequestInit,
): Promise<T> => {
  const response = await fetch(`${getApiBaseUrl()}${path}`, {
    ...init,
    headers: {
      accept: 'application/json',
      ...(init?.headers ?? {}),
      cookie: request.headers.get('cookie') ?? '',
    },
  });

  if (!response.ok) {
    throw new Error(`protected request failed with status ${response.status}`);
  }

  return (await response.json()) as T;
};

export const forwardSetCookieHeaders = (source: Headers, target: Headers): void => {
  const withGetSetCookie = source as Headers & {
    getSetCookie?: () => string[];
  };
  const setCookies =
    typeof withGetSetCookie.getSetCookie === 'function'
      ? withGetSetCookie.getSetCookie()
      : source.get('set-cookie')
        ? [source.get('set-cookie') as string]
        : [];

  for (const cookie of setCookies) {
    target.append('set-cookie', cookie);
  }
};

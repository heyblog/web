import type { ManagementPermissionKey } from './auth.guard';
import { forwardSetCookieHeaders, getApiBaseUrl, getWebPublicBaseUrl } from './auth.server';

type RedirectParams = Record<string, string | null | undefined>;

export const buildRedirectLocation = (pathname: string, params: RedirectParams = {}): string => {
  const target = new URL(pathname, 'http://zhblogs.local');

  for (const [key, value] of Object.entries(params)) {
    if (value) {
      target.searchParams.set(key, value);
    }
  }

  return `${target.pathname}${target.search}`;
};

export const buildRedirectUrl = (
  request: Request,
  pathname: string,
  params: RedirectParams = {},
): string => {
  const location = buildRedirectLocation(pathname, params);
  return new URL(location, resolveRequestOrigin(request)).toString();
};

const INTERNAL_REDIRECT_HOSTS = new Set([
  'api',
  'web',
  'localhost',
  '127.0.0.1',
  '0.0.0.0',
  'host.docker.internal',
  '::1',
  '[::1]',
]);

const isInternalRedirectTarget = (target: URL, apiUrl: URL): boolean =>
  target.origin === apiUrl.origin || INTERNAL_REDIRECT_HOSTS.has(target.hostname);

const readFirstForwardedValue = (request: Request, name: string): string | null => {
  const value = request.headers.get(name)?.split(',')[0]?.trim();
  return value || null;
};

export const resolveRequestOrigin = (request: Request): string => {
  const configuredPublicBaseUrl = getWebPublicBaseUrl();
  if (configuredPublicBaseUrl) {
    return configuredPublicBaseUrl;
  }

  const requestUrl = new URL(request.url);
  const forwardedHost =
    readFirstForwardedValue(request, 'x-forwarded-host') ||
    readFirstForwardedValue(request, 'host');

  if (!forwardedHost) {
    return requestUrl.origin;
  }

  const forwardedProto =
    readFirstForwardedValue(request, 'x-forwarded-proto') || requestUrl.protocol;
  const protocol = forwardedProto.replace(/:$/, '');

  return `${protocol}://${forwardedHost}`;
};

export const resolveUpstreamRedirectLocation = (
  request: Request,
  location: string | null,
  fallbackPath = '/login',
  fallbackParams: RedirectParams = { error: 'request_failed' },
): string => {
  if (!location) {
    return buildRedirectUrl(request, fallbackPath, fallbackParams);
  }

  try {
    const requestUrl = new URL(resolveRequestOrigin(request));
    const apiUrl = new URL(getApiBaseUrl());
    const target = new URL(location, requestUrl);

    if (isInternalRedirectTarget(target, apiUrl)) {
      return new URL(`${target.pathname}${target.search}${target.hash}`, requestUrl).toString();
    }

    return target.toString();
  } catch {
    return buildRedirectUrl(request, fallbackPath, fallbackParams);
  }
};

const isManagementPath = (path: string): boolean =>
  path === '/management' || path.startsWith('/management/');

type LoginUser = {
  role: 'USER' | 'ADMIN' | 'SYS_ADMIN';
  permissions: ManagementPermissionKey[];
};

export const resolvePostLoginRedirect = (nextPath: string | null, _user: LoginUser): string => {
  if (nextPath && !isManagementPath(nextPath)) {
    return nextPath;
  }

  return '/dashboard';
};

export const sanitizeNextPath = (
  value: FormDataEntryValue | string | null | undefined,
): string | null => {
  if (typeof value !== 'string') {
    return null;
  }

  const normalized = value.trim();

  if (!normalized.startsWith('/') || normalized.startsWith('//')) {
    return null;
  }

  return normalized;
};

export const readApiErrorCode = async (response: Response): Promise<string> => {
  try {
    const payload = (await response.json()) as {
      code?: string;
    };

    return payload.code ?? 'request_failed';
  } catch {
    return 'request_failed';
  }
};

export const proxyAuthJson = async (
  request: Request,
  path: string,
  body: Record<string, unknown>,
  method = 'POST',
): Promise<Response> =>
  fetch(`${getApiBaseUrl()}${path}`, {
    method,
    headers: {
      accept: 'application/json',
      'content-type': 'application/json',
      cookie: request.headers.get('cookie') ?? '',
    },
    body: JSON.stringify(body),
  });

export const createRedirectHeaders = (response?: Response): Headers => {
  const headers = new Headers();

  if (response) {
    forwardSetCookieHeaders(response.headers, headers);
  }

  return headers;
};

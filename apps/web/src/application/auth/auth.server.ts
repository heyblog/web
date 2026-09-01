import { loadWebServerConfig } from '../../config.server.ts';

import type { ProblemDetails, SessionUser } from './auth.types';

const forwardResponseHeaders = ['content-type', 'www-authenticate'] as const;

export async function requestAuthAPI(
  request: Request,
  path: `/${string}`,
  init: Readonly<{
    method?: 'GET' | 'POST' | 'PATCH';
    body?: Readonly<Record<string, unknown>>;
  }> = {},
): Promise<Response> {
  const configuration = loadWebServerConfig();
  const headers = new Headers({
    Accept: 'application/json',
    'X-HeyBlog-Web-Token': configuration.apiWebToken,
  });
  const cookie = request.headers.get('cookie');
  if (cookie) headers.set('Cookie', cookie);
  if (init.body) headers.set('Content-Type', 'application/json');

  let upstream: Response;
  try {
    upstream = await fetch(new URL(path, configuration.apiBaseUrl), {
      method: init.method ?? 'GET',
      headers,
      body: init.body ? JSON.stringify(init.body) : undefined,
      redirect: 'manual',
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(10_000)]),
    });
  } catch {
    return Response.json(
      { code: 'bad_gateway', detail: '登录服务暂时不可用', status: 502 },
      { status: 502, headers: { 'Cache-Control': 'no-store' } },
    );
  }

  const responseHeaders = new Headers({ 'Cache-Control': 'no-store' });
  for (const name of forwardResponseHeaders) {
    const value = upstream.headers.get(name);
    if (value) responseHeaders.set(name, value);
  }
  for (const cookieValue of upstream.headers.getSetCookie()) {
    responseHeaders.append('Set-Cookie', cookieValue);
  }
  const location = upstream.headers.get('location');
  if (location) responseHeaders.set('Location', location);
  return new Response(upstream.body, { status: upstream.status, headers: responseHeaders });
}

export async function readSessionUser(request: Request): Promise<SessionUser | null> {
  if (!request.headers.get('cookie')) return null;
  const response = await requestAuthAPI(request, '/auth/me');
  if (response.status === 401) return null;
  if (!response.ok) throw new Error('failed to resolve session user');
  const payload = (await response.json()) as { readonly user: SessionUser };
  return payload.user;
}

export async function readProblemCode(response: Response): Promise<string> {
  try {
    const problem = (await response.json()) as ProblemDetails;
    return problem.code || 'request_failed';
  } catch {
    return 'request_failed';
  }
}

export function copySetCookie(source: Response, target: Headers): void {
  for (const cookie of source.headers.getSetCookie()) target.append('Set-Cookie', cookie);
}

export function safeNext(value: FormDataEntryValue | string | null | undefined): string {
  if (typeof value !== 'string') return '/dashboard';
  const path = value.trim();
  return path.startsWith('/') && !path.startsWith('//') ? path : '/dashboard';
}

export function pageLocation(request: Request, path: string): string {
  return new URL(path, request.url).toString();
}

export type OAuthRoute = 'github/start' | 'github/callback';

export function resolveOAuthLocation(
  request: Request,
  route: OAuthRoute,
  location: string | null,
): string | null {
  if (!location) return null;
  let target: URL;
  try {
    target = new URL(location, request.url);
  } catch {
    return null;
  }
  if (route === 'github/start') {
    return target.origin === 'https://github.com' ? target.toString() : null;
  }
  return new URL(target.pathname + target.search, request.url).toString();
}

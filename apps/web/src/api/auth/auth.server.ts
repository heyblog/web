import { loadWebServerConfig } from '../../config.server.ts';
import { forwardClientAddress } from '../transport/client-ip.server.ts';

import type { ProblemDetails } from './auth.types.ts';

const forwardResponseHeaders = [
  'content-type',
  'www-authenticate',
  'retry-after',
  'ratelimit-limit',
  'ratelimit-remaining',
  'ratelimit-reset',
] as const;

export async function requestAuthAPI(
  request: Request,
  path: `/${string}`,
  init: Readonly<{
    method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE';
    body?: Readonly<Record<string, unknown>>;
  }> = {},
): Promise<Response> {
  const configuration = loadWebServerConfig();
  const upstreamURL = new URL(path, configuration.apiBaseUrl);
  const timeoutMs = upstreamURL.pathname === '/auth/github/callback' ? 55_000 : 10_000;
  const headers = new Headers({
    Accept: 'application/json',
    'X-HeyBlog-Web-Token': configuration.apiWebToken,
  });
  const cookie = request.headers.get('cookie');
  if (cookie) headers.set('Cookie', cookie);
  if (init.body) headers.set('Content-Type', 'application/json');
  forwardClientAddress(request, headers);

  let upstream: Response;
  try {
    upstream = await fetch(upstreamURL, {
      method: init.method ?? 'GET',
      headers,
      body: init.body ? JSON.stringify(init.body) : undefined,
      redirect: 'manual',
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(timeoutMs)]),
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

import { loadWebServerConfig } from '../../config.server.ts';
import { forwardClientAddress } from '../transport/client-ip.server.ts';
import { resolveRequestID } from '../transport/request-id.server.ts';
import { parseRequestID } from '../transport/request-id.ts';

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
    timeoutMs?: number;
  }> = {},
): Promise<Response> {
  const configuration = loadWebServerConfig();
  const upstreamURL = new URL(path, configuration.apiBaseUrl);
  const requestID = resolveRequestID(request);
  const started = performance.now();
  const oauthPhase = /^\/api\/web\/auth\/(github|blogsclub)\/(start|callback)$/u.exec(
    upstreamURL.pathname,
  );
  const timeoutMs =
    init.timeoutMs ??
    (/^\/api\/web\/auth\/(github|blogsclub)\/callback$/u.test(upstreamURL.pathname)
      ? 55_000
      : 10_000);
  const headers = new Headers({
    Accept: 'application/json',
    'X-HeyBlog-Web-Token': configuration.apiWebToken,
    'X-Request-ID': requestID,
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
    if (oauthPhase)
      console.warn('OAuth upstream unavailable', {
        event: 'oauth_upstream',
        provider: oauthPhase[1],
        stage: oauthPhase[2],
        request_id: requestID,
        status: 502,
        duration_ms: Math.round(performance.now() - started),
      });
    return Response.json(
      { code: 'bad_gateway', detail: '登录服务暂时不可用', status: 502, request_id: requestID },
      { status: 502, headers: { 'Cache-Control': 'no-store', 'X-Request-ID': requestID } },
    );
  }

  const responseHeaders = new Headers({ 'Cache-Control': 'no-store' });
  responseHeaders.set(
    'X-Request-ID',
    parseRequestID(upstream.headers.get('X-Request-ID')) ?? requestID,
  );
  if (oauthPhase)
    console.info('OAuth upstream completed', {
      event: 'oauth_upstream',
      provider: oauthPhase[1],
      stage: oauthPhase[2],
      request_id: requestID,
      status: upstream.status,
      duration_ms: Math.round(performance.now() - started),
    });
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

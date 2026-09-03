import { randomBytes } from 'node:crypto';

import { loadWebServerConfig, type WebServerConfig } from '../../config.server.ts';
import { forwardClientAddress } from '../api/client-ip.server.ts';

const requestIDPattern = /^[A-Za-z0-9][A-Za-z0-9._-]{7,63}$/;

export interface SiteSubmissionProxyDependencies {
  loadConfig: () => WebServerConfig;
  fetch: typeof globalThis.fetch;
}

const defaultDependencies: SiteSubmissionProxyDependencies = {
  loadConfig: loadWebServerConfig,
  fetch: globalThis.fetch,
};

export async function forwardSiteSubmission(
  request: Request,
  upstreamPath: `/${string}`,
  method: 'GET' | 'POST',
  dependencies: SiteSubmissionProxyDependencies = defaultDependencies,
): Promise<Response> {
  const requestID = resolveRequestID(request);
  if (request.method !== method) return problem(405, 'method_not_allowed', requestID);
  if (!isSameOriginBrowserRequest(request)) return problem(403, 'forbidden', requestID);
  const configuration = dependencies.loadConfig();
  const incomingURL = new URL(request.url);
  const upstreamURL = new URL(upstreamPath, configuration.apiBaseUrl);
  upstreamURL.search = incomingURL.search;
  const headers = new Headers({
    Accept: 'application/json',
    'X-HeyBlog-Web-Token': configuration.apiWebToken,
    'X-Request-ID': requestID,
  });
  forwardClientAddress(request, headers);
  let body: BodyInit | undefined;
  if (method === 'POST') {
    const mediaType = request.headers.get('content-type')?.split(';', 1)[0]?.trim().toLowerCase();
    if (mediaType !== 'application/json') {
      return problem(415, 'unsupported_media_type', requestID);
    }
    const bytes = await request.arrayBuffer();
    if (bytes.byteLength > 256_000) return problem(413, 'request_too_large', requestID);
    headers.set('Content-Type', 'application/json');
    body = bytes;
  }
  try {
    const upstream = await dependencies.fetch(upstreamURL, {
      method,
      headers,
      body,
      redirect: 'manual',
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(10_000)]),
    });
    const responseHeaders = new Headers({
      'Cache-Control': 'no-store',
      'Content-Type': upstream.headers.get('content-type') ?? 'application/json',
      'X-Request-ID': upstream.headers.get('X-Request-ID') ?? requestID,
    });
    return new Response(upstream.body, { status: upstream.status, headers: responseHeaders });
  } catch {
    return problem(502, 'bad_gateway', requestID);
  }
}

function isSameOriginBrowserRequest(request: Request): boolean {
  const site = request.headers.get('Sec-Fetch-Site');
  const destination = request.headers.get('Sec-Fetch-Dest');
  return site === 'same-origin' && (destination === 'empty' || destination === 'document');
}

function resolveRequestID(request: Request): string {
  const value = request.headers.get('X-Request-ID') ?? '';
  return requestIDPattern.test(value) ? value : randomBytes(16).toString('hex');
}

function problem(status: number, code: string, requestID: string): Response {
  return Response.json(
    {
      type: `urn:heyblog:problem:${code}`,
      status,
      code,
      detail: '请求无法处理',
      request_id: requestID,
    },
    {
      status,
      headers: {
        'Cache-Control': 'no-store',
        'Content-Type': 'application/problem+json',
        'X-Request-ID': requestID,
      },
    },
  );
}

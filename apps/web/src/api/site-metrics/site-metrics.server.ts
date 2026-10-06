import type { WebServerConfig } from '../../config.server.ts';
import { loadWebServerConfig } from '../../config.server.ts';
import { forwardClientAddress } from '../transport/client-ip.server.ts';
import { apiWebTokenHeader } from '../transport/endpoint.server.ts';

import { parseImpressions } from './site-metrics.types.ts';

export interface SiteMetricsDependencies {
  readonly fetch?: typeof fetch;
  readonly loadConfig?: () => WebServerConfig;
}

export function metricProblem(status: number): Response {
  return Response.json(
    { status, code: status === 404 ? 'not_found' : 'request_failed', detail: '请求无法处理' },
    {
      status,
      headers: { 'Cache-Control': 'no-store', 'Referrer-Policy': 'origin' },
    },
  );
}

export function isSameOriginEvidence(request: Request): boolean {
  const url = new URL(request.url);
  const origin = request.headers.get('Origin');
  if (origin && origin !== url.origin) return false;
  const site = request.headers.get('Sec-Fetch-Site');
  if (site) return site === 'same-origin';
  try {
    return new URL(request.headers.get('Referer') ?? '').origin === url.origin;
  } catch {
    return false;
  }
}

async function postMetric(
  request: Request,
  path: string,
  body: unknown,
  dependencies: SiteMetricsDependencies,
): Promise<Response> {
  try {
    const config = (dependencies.loadConfig ?? loadWebServerConfig)();
    const headers = new Headers({
      Accept: 'application/json',
      'Content-Type': 'application/json',
      [apiWebTokenHeader]: config.apiWebToken,
    });
    forwardClientAddress(request, headers);
    return await (dependencies.fetch ?? fetch)(new URL(path, config.apiBaseUrl), {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
      redirect: 'error',
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(5_000)]),
    });
  } catch {
    return metricProblem(503);
  }
}

export async function resolveSiteOutbound(
  request: Request,
  shortId: string,
  eventId: string | null,
  dependencies: SiteMetricsDependencies = {},
): Promise<
  | { readonly kind: 'success'; readonly homepageUrl: string }
  | { readonly kind: 'failure'; readonly status: number }
> {
  const response = await postMetric(
    request,
    `/sites/id/${encodeURIComponent(shortId)}/outbound`,
    { eventId },
    dependencies,
  );
  if (!response.ok) return { kind: 'failure', status: response.status === 404 ? 404 : 503 };
  const value: unknown = await response.json().catch(() => null);
  if (
    !value ||
    typeof value !== 'object' ||
    !('homepageUrl' in value) ||
    typeof value.homepageUrl !== 'string'
  )
    return { kind: 'failure', status: 503 };
  return { kind: 'success', homepageUrl: value.homepageUrl };
}

export async function handleImpressions(
  request: Request,
  dependencies: SiteMetricsDependencies = {},
): Promise<Response> {
  if (request.method !== 'POST') return metricProblem(405);
  if (
    !isSameOriginEvidence(request) ||
    (request.headers.get('Sec-Fetch-Dest') ?? 'empty') !== 'empty'
  )
    return metricProblem(403);
  if (
    new URL(request.url).search ||
    request.headers.get('Content-Type')?.split(';')[0]?.trim() !== 'application/json'
  )
    return metricProblem(400);
  const reader = request.body?.getReader();
  if (!reader) return metricProblem(400);
  const chunks: Uint8Array[] = [];
  let length = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      length += value.byteLength;
      if (length > 32_000) {
        await reader.cancel();
        return metricProblem(413);
      }
      chunks.push(value);
    }
  } catch {
    return metricProblem(400);
  }
  const bytes = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  let value: unknown;
  try {
    value = JSON.parse(new TextDecoder().decode(bytes));
  } catch {
    return metricProblem(400);
  }
  const events = parseImpressions(value);
  if (!events) return metricProblem(400);
  const upstream = await postMetric(request, '/site-metrics/impressions', { events }, dependencies);
  const headers = new Headers({ 'Cache-Control': 'no-store' });
  for (const name of ['Retry-After', 'RateLimit-Limit', 'RateLimit-Remaining', 'RateLimit-Reset']) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }
  if (upstream.status === 204) return new Response(null, { status: 204, headers });
  const status = [400, 401, 403, 413, 422, 429].includes(upstream.status) ? upstream.status : 503;
  return new Response(metricProblem(status).body, {
    status,
    headers: { ...Object.fromEntries(headers), 'Content-Type': 'application/json' },
  });
}

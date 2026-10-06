import { randomUUID } from 'node:crypto';

import {
  isSameOriginEvidence,
  metricProblem,
  resolveSiteOutbound,
  type SiteMetricsDependencies,
} from '../../api/site-metrics/site-metrics.server.ts';

import { attributedSiteUrl } from './site-outbound.shared.ts';

export function isCountedNavigation(request: Request): boolean {
  if (request.method !== 'GET' || !isSameOriginEvidence(request)) return false;
  if (
    /prefetch|prerender/iu.test(
      `${request.headers.get('Purpose') ?? ''} ${request.headers.get('Sec-Purpose') ?? ''}`,
    )
  )
    return false;
  const mode = request.headers.get('Sec-Fetch-Mode');
  const destination = request.headers.get('Sec-Fetch-Dest');
  return (!mode || mode === 'navigate') && (!destination || destination === 'document');
}

export async function handleSiteOutbound(
  request: Request,
  shortId: string,
  dependencies: SiteMetricsDependencies = {},
): Promise<Response> {
  if (request.method !== 'GET' && request.method !== 'HEAD') return metricProblem(405);
  if (!/^[0-9A-Za-z]{9}$/u.test(shortId) || new URL(request.url).search) return metricProblem(404);
  const result = await resolveSiteOutbound(
    request,
    shortId,
    isCountedNavigation(request) ? randomUUID() : null,
    dependencies,
  );
  if (result.kind === 'failure') return metricProblem(result.status);
  const target = attributedSiteUrl(result.homepageUrl);
  if (!target) return metricProblem(503);
  return new Response(null, {
    status: 302,
    headers: {
      Location: target,
      'Cache-Control': 'no-store',
      'Referrer-Policy': 'origin',
      'X-Robots-Tag': 'noindex, nofollow',
    },
  });
}

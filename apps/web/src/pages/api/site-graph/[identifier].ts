import type { APIRoute } from 'astro';

import { createApiEndpoint } from '../../../api/transport/endpoint.server.ts';

export const GET: APIRoute = (context) => {
  const identifier = context.params.identifier ?? '';
  if (!/^[A-Za-z0-9-]{1,36}$/.test(identifier))
    return new Response(null, { status: 400, headers: { 'Cache-Control': 'no-store' } });
  return createApiEndpoint({
    audience: 'web-only',
    method: 'GET',
    upstreamPath: `/sites/id/${identifier}/graph`,
    responseHeaders: ['content-type'],
    timeoutMs: 30_000,
  }).GET(context);
};
export const prerender = false;

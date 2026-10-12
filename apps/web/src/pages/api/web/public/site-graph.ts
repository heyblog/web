import { createApiEndpoint } from '@/api/transport/endpoint.server.ts';

const endpoint = createApiEndpoint({
  audience: 'web-only',
  method: 'GET',
  upstreamPath: '/api/web/public/sites/graph',
  responseHeaders: ['content-type', 'content-encoding', 'vary'],
  preserveEncoding: true,
  timeoutMs: 30_000,
});
export const GET = endpoint.GET;
export const OPTIONS = endpoint.OPTIONS;
export const prerender = false;

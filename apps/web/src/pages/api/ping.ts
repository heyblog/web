import { createApiEndpoint } from '../../api/transport/endpoint.server.ts';

const endpoint = createApiEndpoint({
  audience: 'web-only',
  method: 'GET',
  upstreamPath: '/ping',
  responseHeaders: ['content-type'],
});

export const GET = endpoint.GET;
export const OPTIONS = endpoint.OPTIONS;
export const prerender = false;

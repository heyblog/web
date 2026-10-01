import { createApiEndpoint } from '../../api/transport/endpoint.server.ts';

export const prerender = false;
export const { GET, OPTIONS } = createApiEndpoint({
  audience: 'web-only',
  method: 'GET',
  upstreamPath: '/sites/random',
  queryParameters: ['level1', 'level2'],
  responseHeaders: ['content-type'],
});

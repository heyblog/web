import { createApiEndpoint } from '@/application/api/endpoint.server';

const endpoint = createApiEndpoint({
  audience: 'web-only',
  method: 'GET',
  upstreamPath: '/sites',
  queryParameters: [
    'page',
    'q',
    'primary',
    'secondary',
    'warning',
    'technology',
    'access',
    'feed',
    'status',
    'sort',
    'order',
    'seed',
  ],
  responseHeaders: ['content-type'],
});

export const GET = endpoint.GET;
export const OPTIONS = endpoint.OPTIONS;
export const prerender = false;

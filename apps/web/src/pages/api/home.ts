import { createApiEndpoint } from '@/application/api/endpoint.server';

const endpoint = createApiEndpoint({
  audience: 'web-only',
  method: 'GET',
  upstreamPath: '/home',
  responseHeaders: ['content-type'],
});

export const GET = endpoint.GET;
export const OPTIONS = endpoint.OPTIONS;
export const prerender = false;

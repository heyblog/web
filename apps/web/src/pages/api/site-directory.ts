import { createApiEndpoint } from '../../api/transport/endpoint.server.ts';

const endpoint = createApiEndpoint({
  audience: 'web-only',
  method: 'GET',
  upstreamPath: '/sites',
  queryParameters: [
    'page',
    'q',
    'level1',
    'level2',
    'tertiary',
    'level1_label_id',
    'level2_label_id',
    'tertiary_label_id',
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

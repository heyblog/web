import type { APIRoute } from 'astro';

import { getApiBaseUrl } from '@/application/auth/auth.server';
import {
  createRedirectHeaders,
  resolveUpstreamRedirectLocation,
} from '@/application/auth/auth-route.server';

export const prerender = false;

export const GET: APIRoute = async ({ request }) => {
  const response = await fetch(`${getApiBaseUrl()}/auth/github/start`, {
    headers: {
      accept: 'text/html,*/*',
      cookie: request.headers.get('cookie') ?? '',
    },
    redirect: 'manual',
  });
  const headers = createRedirectHeaders(response);

  headers.set(
    'Location',
    resolveUpstreamRedirectLocation(request, response.headers.get('location')),
  );

  return new Response(null, {
    status: 302,
    headers,
  });
};

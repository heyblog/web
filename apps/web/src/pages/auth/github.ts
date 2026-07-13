import type { APIRoute } from 'astro';

import { getApiBaseUrl } from '@/application/auth/auth.server';
import {
  createRedirectHeaders,
  resolveUpstreamRedirectLocation,
  sanitizeNextPath,
} from '@/application/auth/auth-route.server';

export const prerender = false;

export const GET: APIRoute = async ({ request, url }) => {
  const target = new URL(`${getApiBaseUrl()}/auth/github`);
  const nextPath = sanitizeNextPath(url.searchParams.get('next'));

  if (nextPath) {
    target.searchParams.set('next', nextPath);
  }

  const response = await fetch(target, {
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

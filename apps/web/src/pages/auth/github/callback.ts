import type { APIRoute } from 'astro';

import { getApiBaseUrl } from '@/application/auth/auth.server';
import {
  buildRedirectUrl,
  createRedirectHeaders,
  resolveUpstreamRedirectLocation,
} from '@/application/auth/auth-route.server';

export const prerender = false;

export const GET: APIRoute = async ({ request, url }) => {
  if (!url.searchParams.get('code') || !url.searchParams.get('state')) {
    return Response.redirect(buildRedirectUrl(request, '/login', { error: 'request_failed' }), 302);
  }

  const response = await fetch(
    `${getApiBaseUrl()}/auth/github/exchange?${url.searchParams.toString()}`,
    {
      headers: {
        accept: 'application/json',
        cookie: request.headers.get('cookie') ?? '',
      },
      redirect: 'manual',
    },
  );

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

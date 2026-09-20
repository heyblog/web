import type { APIRoute } from 'astro';

import { requestAuthAPI } from '@/application/auth/auth.server';

export const prerender = false;

export const GET: APIRoute = ({ request }) => {
  if (request.headers.get('Sec-Fetch-Site') !== 'same-origin')
    return new Response(null, { status: 403 });
  return requestAuthAPI(request, '/management/api-clients');
};

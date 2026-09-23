import type { APIRoute } from 'astro';

import { legacyRandomDestination } from '@/application/site-go/site-go.shared';

export const prerender = false;

export const GET: APIRoute = ({ request }) =>
  new Response(null, {
    status: 301,
    headers: { Location: legacyRandomDestination(new URL(request.url)) },
  });

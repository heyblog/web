import type { APIRoute } from 'astro';

import { handleSiteOutbound } from '@/application/site-outbound/site-outbound.server';

export const prerender = false;
export const ALL: APIRoute = ({ request, params }) =>
  handleSiteOutbound(request, params.shortId ?? '');

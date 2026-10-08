import type { APIRoute } from 'astro';

import { forwardSiteManagement } from '../../../api/site-management/proxy.server.ts';

export const prerender = false;
export const ALL: APIRoute = ({ request, params }) =>
  forwardSiteManagement(request, params.path ?? '');

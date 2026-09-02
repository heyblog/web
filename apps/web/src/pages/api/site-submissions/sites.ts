import type { APIRoute } from 'astro';

import { forwardSiteSubmission } from '@/application/site-submission/proxy.server';

export const GET: APIRoute = ({ request }) =>
  forwardSiteSubmission(request, '/site-submissions/sites', 'GET');

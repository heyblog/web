import type { APIRoute } from 'astro';

import { forwardSiteSubmission } from '@/application/site-submission/proxy.server';

export const POST: APIRoute = ({ request }) =>
  forwardSiteSubmission(request, '/site-submissions', 'POST');

import type { APIRoute } from 'astro';

import { forwardSiteSubmission } from '../../../api/site-submission/proxy.server.ts';

export const GET: APIRoute = ({ request }) =>
  forwardSiteSubmission(request, '/site-submissions/options', 'GET');

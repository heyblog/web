import type { APIRoute } from 'astro';

import { forwardSiteSubmission } from '../../../api/site-submission/proxy.server.ts';

export const POST: APIRoute = ({ request }) =>
  forwardSiteSubmission(request, '/site-submissions/query', 'POST');

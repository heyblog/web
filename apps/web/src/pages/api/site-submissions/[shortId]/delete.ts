import type { APIRoute } from 'astro';

import { forwardSiteSubmission } from '@/application/site-submission/proxy.server';
import { isSiteShortID } from '@/application/site-submission/site-submission.validation';

export const POST: APIRoute = ({ request, params }) => {
  const shortID = params.shortId ?? '';
  if (!isSiteShortID(shortID)) return new Response(null, { status: 404 });
  return forwardSiteSubmission(request, `/site-submissions/${shortID}/deletions`, 'POST');
};

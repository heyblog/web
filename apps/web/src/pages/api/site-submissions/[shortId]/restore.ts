import type { APIRoute } from 'astro';

import { forwardSiteSubmission } from '../../../../api/site-submission/proxy.server.ts';
import { isSiteShortID } from '../../../../application/site-submission/site-submission.validation.ts';

export const POST: APIRoute = ({ request, params }) => {
  const shortID = params.shortId ?? '';
  if (!isSiteShortID(shortID)) return new Response(null, { status: 404 });
  return forwardSiteSubmission(request, `/site-submissions/${shortID}/restorations`, 'POST');
};

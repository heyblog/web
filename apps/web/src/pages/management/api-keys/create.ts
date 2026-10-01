import type { APIRoute } from 'astro';

import {
  createApiClient,
  invalidRequest,
  isSameOriginMutation,
  readMutationBody,
} from '../../../api/api-keys/api-keys.server.ts';
import { parseApiClientCreatePayload } from '../../../api/api-keys/api-keys.types.ts';

export const prerender = false;

export const POST: APIRoute = async ({ request }) => {
  if (!isSameOriginMutation(request)) return new Response(null, { status: 403 });
  const body = await readMutationBody(request);
  const payload = parseApiClientCreatePayload(body);
  if (payload === null) return invalidRequest();
  return createApiClient(request, {
    name: payload.name,
    description: payload.description,
    audience: payload.audience,
    scopes: payload.scopes,
    expires_at: payload.expires_at,
    never_expires: payload.never_expires,
  });
};

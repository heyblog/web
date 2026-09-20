import type { APIRoute } from 'astro';

import {
  invalidRequest,
  isSameOriginMutation,
  isUUID,
  readMutationBody,
  updateApiClient,
} from '@/application/api-keys/api-keys.server';
import { parseApiClientUpdatePayload } from '@/application/api-keys/api-keys.types';

export const prerender = false;

export const POST: APIRoute = async ({ params, request }) => {
  if (!isSameOriginMutation(request)) return new Response(null, { status: 403 });
  if (!isUUID(params.clientId)) return invalidRequest();
  const body = await readMutationBody(request);
  const payload = parseApiClientUpdatePayload(body);
  if (payload === null) return invalidRequest();
  return updateApiClient(request, params.clientId, {
    name: payload.name,
    description: payload.description,
    scopes: payload.scopes,
    enabled: payload.enabled,
  });
};

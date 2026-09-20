import type { APIRoute } from 'astro';

import {
  invalidRequest,
  isRecord,
  isSameOriginMutation,
  isUUID,
  readMutationBody,
  rotateApiClient,
} from '@/application/api-keys/api-keys.server';

export const prerender = false;

export const POST: APIRoute = async ({ params, request }) => {
  if (!isSameOriginMutation(request)) return new Response(null, { status: 403 });
  if (!isUUID(params.clientId)) return invalidRequest();
  const body = await readMutationBody(request);
  if (!isRecord(body) || typeof body.overlap_hours !== 'number') return invalidRequest();
  const overlapHours = body.overlap_hours;
  if (!Number.isInteger(overlapHours) || overlapHours < 0 || overlapHours > 24)
    return invalidRequest();
  return rotateApiClient(request, params.clientId, overlapHours);
};

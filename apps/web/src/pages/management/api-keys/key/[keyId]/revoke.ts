import type { APIRoute } from 'astro';

import {
  invalidRequest,
  isSameOriginMutation,
  isUUID,
  revokeApiKey,
} from '@/application/api-keys/api-keys.server';

export const prerender = false;

export const POST: APIRoute = ({ params, request }) => {
  if (!isSameOriginMutation(request)) return new Response(null, { status: 403 });
  if (!isUUID(params.keyId)) return invalidRequest();
  return revokeApiKey(request, params.keyId);
};

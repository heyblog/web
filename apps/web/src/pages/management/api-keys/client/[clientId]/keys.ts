import type { APIRoute } from 'astro';

import {
  invalidRequest,
  isSameOriginMutation,
  issueApiKey,
  isUUID,
  readMutationBody,
} from '@/application/api-keys/api-keys.server';
import { parseApiKeyIssuePayload } from '@/application/api-keys/api-keys.types';

export const prerender = false;

export const POST: APIRoute = async ({ params, request }) => {
  if (!isSameOriginMutation(request)) return new Response(null, { status: 403 });
  if (!isUUID(params.clientId)) return invalidRequest();
  const payload = parseApiKeyIssuePayload(await readMutationBody(request));
  return payload === null ? invalidRequest() : issueApiKey(request, params.clientId, payload);
};

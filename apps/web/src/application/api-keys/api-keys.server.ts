import { requestAuthAPI } from '@/application/auth/auth.server';

import { parseClients } from './api-keys.responses';
import type { ApiClientsResponse, ApiKeyIssuePayload } from './api-keys.types';

export function isSameOriginMutation(request: Request): boolean {
  return request.headers.get('Sec-Fetch-Site') === 'same-origin';
}

export function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function isUUID(value: string | undefined): value is string {
  return Boolean(
    value &&
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value),
  );
}

export function invalidRequest(): Response {
  return Response.json(
    { code: 'validation_failed', detail: '请求内容无效', status: 422 },
    { status: 422, headers: { 'Cache-Control': 'no-store' } },
  );
}

export async function listApiClients(request: Request): Promise<ApiClientsResponse> {
  const response = await requestAuthAPI(request, '/management/api-clients');
  if (!response.ok) throw new Error('failed to list API clients');
  const clients = parseClients(await response.json());
  if (clients === null) throw new Error('invalid API clients response');
  return { clients };
}

export async function createApiClient(
  request: Request,
  body: Readonly<Record<string, unknown>>,
): Promise<Response> {
  return requestAuthAPI(request, '/management/api-clients', { method: 'POST', body });
}

export async function updateApiClient(
  request: Request,
  clientId: string,
  body: Readonly<Record<string, unknown>>,
): Promise<Response> {
  return requestAuthAPI(request, `/management/api-clients/${clientId}`, {
    method: 'PATCH',
    body,
  });
}

export async function rotateApiClient(
  request: Request,
  clientId: string,
  overlapHours: number,
): Promise<Response> {
  return requestAuthAPI(request, `/management/api-clients/${clientId}/rotate`, {
    method: 'POST',
    body: { overlap_hours: overlapHours },
  });
}

export async function revokeApiKey(request: Request, keyId: string): Promise<Response> {
  return requestAuthAPI(request, `/management/api-keys/${keyId}/revoke`, { method: 'POST' });
}

export async function issueApiKey(
  request: Request,
  clientId: string,
  body: ApiKeyIssuePayload,
): Promise<Response> {
  return requestAuthAPI(request, `/management/api-clients/${clientId}/keys`, {
    method: 'POST',
    body: { ...body },
  });
}

export async function readMutationBody(request: Request): Promise<unknown> {
  try {
    return await request.json();
  } catch (error) {
    if (error instanceof SyntaxError) return null;
    throw error;
  }
}

import { requestAuthAPI } from '../auth/auth.server.ts';
import { isWebPageFetch } from '../transport/endpoint.server.ts';

import { isEmailChangeId } from './email-change.types.ts';

interface ProfileRoute {
  readonly method: 'GET' | 'POST' | 'PATCH' | 'DELETE';
  readonly fields?: readonly string[];
  readonly emailChange?: true;
}

function profileRoutes(path: string): readonly ProfileRoute[] {
  switch (path) {
    case 'profile':
      return [{ method: 'PATCH', fields: ['display_name'] }];
    case 'profile/confirm':
      return [{ method: 'POST', fields: ['username', 'display_name'] }];
    case 'username':
      return [{ method: 'POST', fields: ['username'] }];
    case 'email-change':
      return [
        { method: 'GET' },
        { method: 'POST', fields: ['email'] },
        { method: 'DELETE', emailChange: true },
      ];
    case 'email-change/send-code':
      return [{ method: 'POST', fields: ['side'], emailChange: true }];
    case 'email-change/verify-code':
      return [{ method: 'POST', fields: ['side', 'code'], emailChange: true }];
    case 'email-change/verify-password':
      return [{ method: 'POST', fields: ['password'], emailChange: true }];
    case 'email-change/complete':
      return [{ method: 'POST', fields: [], emailChange: true }];
    default:
      return [];
  }
}

export function acceptsProfilePath(path: string): boolean {
  return profileRoutes(path).length > 0;
}

function problem(status: number, code: string): Response {
  return Response.json(
    { status, code },
    { status, headers: { 'Cache-Control': 'private, no-store' } },
  );
}

export async function forwardProfile(request: Request, path: string): Promise<Response> {
  const routes = profileRoutes(path);
  if (!routes.length) return problem(404, 'not_found');
  const route = routes.find(({ method }) => method === request.method);
  if (!route) return problem(405, 'method_not_allowed');
  const origin = new URL(request.url).origin;
  const sameOrigin =
    request.headers.get('Origin') === origin &&
    (!request.headers.has('Sec-Fetch-Site') ||
      request.headers.get('Sec-Fetch-Site') === 'same-origin');
  if (!(route.method === 'GET' ? isWebPageFetch(request) : sameOrigin))
    return problem(403, 'forbidden');
  const emailChangeId = route.emailChange ? request.headers.get('X-HeyBlog-Email-Change-ID') : null;
  if (route.emailChange && !isEmailChangeId(emailChangeId)) return problem(400, 'invalid_request');
  const body = route.fields ? await readProfileBody(request, route.fields) : undefined;
  if (body instanceof Response) return body;
  const response = await requestAuthAPI(request, `/api/web/account/${path}`, {
    method: route.method,
    ...(body ? { body } : {}),
    ...(emailChangeId ? { emailChangeId } : {}),
  });
  response.headers.delete('Location');
  response.headers.set('Cache-Control', 'private, no-store');
  return response;
}

async function readProfileBody(
  request: Request,
  fields: readonly string[],
): Promise<Readonly<Record<string, unknown>> | Response> {
  if (request.headers.get('Content-Type')?.split(';')[0]?.trim() !== 'application/json')
    return problem(415, 'unsupported_media_type');
  const reader = request.body?.getReader();
  if (!reader) return problem(400, 'invalid_request');
  let text = '';
  let bytes = 0;
  const decoder = new TextDecoder();
  try {
    while (true) {
      const chunk = await reader.read();
      if (chunk.done) break;
      bytes += chunk.value.byteLength;
      if (bytes > 4096) {
        await reader.cancel();
        return problem(413, 'request_too_large');
      }
      text += decoder.decode(chunk.value, { stream: true });
    }
    text += decoder.decode();
  } catch {
    return problem(400, 'invalid_request');
  } finally {
    reader.releaseLock();
  }
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    return problem(400, 'invalid_request');
  }
  if (typeof value !== 'object' || value === null || Array.isArray(value))
    return problem(400, 'invalid_request');
  const body = Object.fromEntries(Object.entries(value));
  if (
    Object.keys(body).length !== fields.length ||
    fields.some((key) => typeof body[key] !== 'string')
  )
    return problem(422, 'invalid_request');
  return body;
}

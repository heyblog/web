import { requestAuthAPI } from '../auth/auth.server.ts';

import { isRecord, isUUID } from './taxonomy.types.ts';
import { validManagementPayload } from './taxonomy.validation.ts';

function problem(status: number, code: string): Response {
  return Response.json(
    { status, code, detail: '请求无法处理' },
    { status, headers: { 'Cache-Control': 'private, no-store' } },
  );
}

async function readBody(request: Request, limit: number): Promise<unknown> {
  if (!request.body) return null;
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let length = 0;
  try {
    for (;;) {
      const part = await reader.read();
      if (part.done) break;
      length += part.value.byteLength;
      if (length > limit) {
        await reader.cancel();
        return null;
      }
      chunks.push(part.value);
    }
  } finally {
    reader.releaseLock();
  }
  const bytes = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  try {
    return JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes));
  } catch (error) {
    if (error instanceof SyntaxError || error instanceof TypeError) return null;
    throw error;
  }
}

export async function forwardTaxonomyManagement(
  request: Request,
  path = '',
  upstream: typeof requestAuthAPI = requestAuthAPI,
): Promise<Response> {
  const actions = path === 'changes/preview' || path === 'changes/apply' || path === 'cascades';
  const allowed =
    path === '' ? ['GET', 'POST'] : actions ? ['POST'] : isUUID(path) ? ['PUT', 'DELETE'] : [];
  if (allowed.length === 0) return problem(404, 'not_found');
  if (!allowed.includes(request.method)) return problem(405, 'method_not_allowed');
  const incomingURL = new URL(request.url);
  if (incomingURL.search !== '') return problem(400, 'bad_request');
  let body: Readonly<Record<string, unknown>> | undefined;
  if (request.method !== 'GET') {
    if (
      request.headers.get('Sec-Fetch-Site') !== 'same-origin' ||
      request.headers.get('Origin') !== incomingURL.origin
    )
      return problem(403, 'forbidden');
    if (
      request.headers.get('Content-Type')?.split(';')[0]?.trim().toLowerCase() !==
      'application/json'
    )
      return problem(415, 'unsupported_media_type');
    const value = await readBody(request, 65536);
    if (!validManagementPayload(value, path, request.method))
      return problem(422, 'validation_failed');
    if (!isRecord(value)) return problem(422, 'validation_failed');
    body = value;
  }
  if (!request.headers.get('cookie')) return problem(401, 'unauthorized');
  const session = await upstream(request, '/auth/me');
  if (!session.ok)
    return problem(
      session.status === 401 || session.status === 403 ? session.status : 503,
      session.status === 401
        ? 'unauthorized'
        : session.status === 403
          ? 'forbidden'
          : 'service_unavailable',
    );
  let actor: unknown;
  try {
    actor = await session.json();
  } catch {
    return problem(502, 'bad_gateway');
  }
  if (!isRecord(actor) || !isRecord(actor.user)) return problem(502, 'bad_gateway');
  const user = actor.user;
  if (!(
    user.role === 'SYS_ADMIN' ||
    (user.role === 'ADMIN' &&
      Array.isArray(user.permissions) &&
      user.permissions.includes('taxonomy.manage'))
  ))
    return problem(403, 'forbidden');
  const target: `/${string}` = actions
    ? `/management/taxonomy/${path}`
    : `/management/taxonomy/tags${path ? `/${path}` : ''}`;
  const method = request.method;
  if (
    method !== 'GET' &&
    method !== 'POST' &&
    method !== 'PUT' &&
    method !== 'DELETE' &&
    method !== 'PATCH'
  )
    return problem(405, 'method_not_allowed');
  const response = await upstream(request, target, { method, body });
  for (const cookie of session.headers.getSetCookie())
    response.headers.append('Set-Cookie', cookie);
  response.headers.set('Cache-Control', 'private, no-store');
  response.headers.delete('Location');
  return response;
}

import { requestAuthAPI } from '../auth/auth.server.ts';

import { isRecord } from './managed-users.responses.ts';
import type { AccountOperation } from './managed-users.types.ts';

async function readBody(request: Request): Promise<unknown> {
  if (!request.body) return null;
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let length = 0;
  try {
    for (;;) {
      const part = await reader.read();
      if (part.done) break;
      length += part.value.byteLength;
      if (length > 8192) {
        await reader.cancel();
        return null;
      }
      chunks.push(part.value);
    }
    const bytes = new Uint8Array(length);
    let offset = 0;
    for (const chunk of chunks) {
      bytes.set(chunk, offset);
      offset += chunk.byteLength;
    }
    return JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes));
  } catch {
    return null;
  } finally {
    reader.releaseLock();
  }
}

export async function forwardManagedUserMutation(
  request: Request,
  operation: AccountOperation | 'create',
  id?: string,
): Promise<Response> {
  const fail = (status: number, code: string) =>
    Response.json({ code, status }, { status, headers: { 'Cache-Control': 'private, no-store' } });
  if (
    request.headers.get('Origin') !== new URL(request.url).origin ||
    request.headers.get('Sec-Fetch-Site') !== 'same-origin'
  )
    return fail(403, 'forbidden');
  if (
    request.headers.get('Content-Type')?.split(';')[0]?.trim().toLowerCase() !== 'application/json'
  )
    return fail(415, 'validation_failed');
  if (operation !== 'create' && (!id || !/^[a-zA-Z0-9][a-zA-Z0-9-]*$/u.test(id)))
    return fail(422, 'validation_failed');
  const value = await readBody(request);
  if (!isRecord(value)) return fail(422, 'validation_failed');
  const body: Record<string, unknown> = {};
  if (operation !== 'create') {
    if (typeof value.updated_at !== 'string' || !value.updated_at || value.updated_at.length > 64)
      return fail(422, 'validation_failed');
    body.updated_at = value.updated_at;
  }
  if (operation === 'profile' || operation === 'create') {
    if (
      typeof value.display_name !== 'string' ||
      !value.display_name.trim() ||
      value.display_name.trim().length > 128 ||
      typeof value.email !== 'string' ||
      value.email.length > 320 ||
      !/^[^\s@]+@[^\s@]+\.[^\s@]+$/u.test(value.email.trim())
    )
      return fail(422, 'validation_failed');
    body.display_name = value.display_name.trim();
    body.email = value.email.trim();
  }
  if (operation === 'create') {
    if (typeof value.username !== 'string' || !/^[a-z0-9_]{3,32}$/u.test(value.username))
      return fail(422, 'validation_failed');
    body.username = value.username;
  }
  if (operation === 'status') {
    if (value.status !== 'ACTIVE' && value.status !== 'SUSPENDED')
      return fail(422, 'validation_failed');
    body.status = value.status;
  }
  const response = await requestAuthAPI(
    request,
    operation === 'create'
      ? '/api/web/management/users'
      : `/api/web/management/users/${id}/${operation}`,
    { method: operation === 'profile' || operation === 'status' ? 'PATCH' : 'POST', body },
  );
  response.headers.set('Cache-Control', 'private, no-store');
  response.headers.delete('Location');
  return response;
}

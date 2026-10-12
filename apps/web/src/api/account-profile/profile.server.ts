import { requestAuthAPI } from '../auth/auth.server.ts';

function problem(status: number, code: string): Response {
  return Response.json(
    { status, code },
    { status, headers: { 'Cache-Control': 'private, no-store' } },
  );
}

export async function forwardProfile(request: Request, path: string): Promise<Response> {
  const method = path === 'profile' ? 'PATCH' : 'POST';
  if (!['profile', 'profile/confirm', 'username'].includes(path)) return problem(404, 'not_found');
  if (request.method !== method) return problem(405, 'method_not_allowed');
  const origin = new URL(request.url).origin;
  if (
    request.headers.get('Origin') !== origin ||
    (request.headers.has('Sec-Fetch-Site') &&
      request.headers.get('Sec-Fetch-Site') !== 'same-origin')
  )
    return problem(403, 'forbidden');
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
  const fields =
    path === 'profile/confirm'
      ? ['username', 'display_name']
      : [path === 'profile' ? 'display_name' : 'username'];
  if (
    Object.keys(body).length !== fields.length ||
    fields.some((key) => typeof body[key] !== 'string')
  )
    return problem(422, 'invalid_request');
  const response = await requestAuthAPI(request, `/api/web/account/${path}`, { method, body });
  response.headers.delete('Location');
  response.headers.set('Cache-Control', 'private, no-store');
  return response;
}

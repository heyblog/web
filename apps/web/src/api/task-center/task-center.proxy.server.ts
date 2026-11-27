import { requestAuthAPI } from '../auth/auth.server.ts';

import { ProtocolError, record } from './parse.ts';
const uuid = '[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}';
export type TaskAudience = 'tasks' | 'request-signing' | 'public-signing';
function methods(scope: TaskAudience, path: string): readonly string[] {
  if (scope === 'public-signing')
    return path === 'keys' ? ['GET'] : path === 'verify' ? ['POST'] : [];
  if (scope === 'request-signing') {
    if (path === 'policy') return ['GET', 'PUT'];
    if (path === 'keys') return ['GET', 'POST'];
    if (path === 'events') return ['GET'];
    return new RegExp(`^keys/${uuid}/(?:publish|activate|revoke)$`, 'iu').test(path)
      ? ['POST']
      : [];
  }
  if (path === 'schedules') return ['GET', 'POST'];
  if (path === 'request-configs' || path === 'runs') return ['GET', 'POST'];
  if (['overview', 'stats', 'workers'].includes(path)) return ['GET'];
  if (path === 'runs/batch') return ['POST'];
  if (new RegExp(`^(?:schedules|request-configs)/${uuid}$`, 'iu').test(path)) return ['PUT'];
  if (new RegExp(`^runs/${uuid}(?:/(?:targets|attempts|events|export))?$`, 'iu').test(path))
    return ['GET'];
  return new RegExp(
    `^(?:schedules/${uuid}/run|request-configs/${uuid}/verification-default|runs/${uuid}/(?:pause|resume|cancel|retry|rerun|archive|unarchive|cleanup)|workers/${uuid}/(?:drain|resume))$`,
    'iu',
  ).test(path)
    ? ['POST']
    : [];
}
function acceptsQuery(path: string, query: URLSearchParams): boolean {
  if (!query.size) return true;
  if (!(
    path === 'runs' ||
    path === 'stats' ||
    new RegExp(`^runs/${uuid}/(?:targets|attempts|events|export)$`, 'iu').test(path)
  ))
    return false;
  const allowed = new Set([
    'page',
    'page_size',
    'kind',
    'state',
    'source',
    'schedule',
    'from',
    'to',
    'archived',
  ]);
  if (path.endsWith('/export')) allowed.add('format');
  for (const [key, value] of query) {
    if (!allowed.has(key) || query.getAll(key).length !== 1 || value.length > 120) return false;
    if (key === 'page' && !/^[1-9]\d{0,6}$/u.test(value)) return false;
    if (key === 'page_size' && (!/^[1-9]\d{0,2}$/u.test(value) || Number(value) > 100))
      return false;
    if (key === 'format' && !['csv', 'json'].includes(value)) return false;
    if (key === 'schedule' && !new RegExp(`^${uuid}$`, 'iu').test(value)) return false;
    if (key === 'archived' && !['true', 'false'].includes(value)) return false;
    if (['kind', 'state', 'source'].includes(key) && !/^[a-z_]+$/u.test(value)) return false;
    if (['from', 'to'].includes(key) && !Number.isFinite(Date.parse(value))) return false;
  }
  return true;
}
function pageRead(request: Request): boolean {
  const url = new URL(request.url),
    site = request.headers.get('Sec-Fetch-Site'),
    mode = request.headers.get('Sec-Fetch-Mode'),
    dest = request.headers.get('Sec-Fetch-Dest');
  if (site !== null || mode !== null || dest !== null)
    return (
      site === 'same-origin' && (mode === 'cors' || mode === 'same-origin') && dest === 'empty'
    );
  return (
    request.headers.get('Accept') === 'application/json' &&
    URL.parse(request.headers.get('Referer') ?? '')?.origin === url.origin &&
    (!request.headers.has('Origin') || request.headers.get('Origin') === url.origin)
  );
}
function failure(status: number, code: string): Response {
  return Response.json(
    { status, code },
    { status, headers: { 'Cache-Control': 'private, no-store' } },
  );
}
export async function forwardTask(
  request: Request,
  scope: TaskAudience,
  path: string,
  forward = requestAuthAPI,
): Promise<Response> {
  const allowed = methods(scope, path);
  if (!allowed.length) return failure(404, 'not_found');
  if (!allowed.includes(request.method)) return failure(405, 'method_not_allowed');
  const url = new URL(request.url);
  if ((request.method !== 'GET' && url.search) || !acceptsQuery(path, url.searchParams))
    return failure(400, 'bad_request');
  if (
    request.method === 'GET'
      ? !pageRead(request)
      : request.headers.get('Origin') !== url.origin ||
        request.headers.get('Sec-Fetch-Site') !== 'same-origin'
  )
    return failure(403, 'forbidden');
  if (scope !== 'public-signing' && !request.headers.has('Cookie'))
    return failure(401, 'unauthorized');
  let body: Record<string, unknown> | undefined;
  if (request.method !== 'GET') {
    if (!/^application\/json(?:\s*;|$)/iu.test(request.headers.get('Content-Type') ?? ''))
      return failure(415, 'unsupported_media_type');
    const reader = request.body?.getReader();
    if (!reader) return failure(422, 'validation_failed');
    let bytes = 0,
      raw = '';
    const decoder = new TextDecoder();
    try {
      for (;;) {
        const chunk = await reader.read();
        if (chunk.done) break;
        bytes += chunk.value.length;
        if (bytes > 65536) {
          await reader.cancel();
          return failure(413, 'request_too_large');
        }
        raw += decoder.decode(chunk.value, { stream: true });
      }
      raw += decoder.decode();
      body = record(JSON.parse(raw));
    } catch (error) {
      if (error instanceof SyntaxError || error instanceof ProtocolError)
        return failure(422, 'validation_failed');
      throw error;
    } finally {
      reader.releaseLock();
    }
  }
  const prefix = scope === 'public-signing' ? 'public/request-signing' : `management/${scope}`;
  const method = request.method === 'PUT' ? 'PUT' : request.method === 'POST' ? 'POST' : 'GET';
  const key = request.headers.get('Idempotency-Key');
  if (key !== null && !/^[a-zA-Z0-9_-]{1,120}$/u.test(key))
    return failure(422, 'validation_failed');
  const response = await forward(request, `/api/web/${prefix}/${path}${url.search}`, {
    method,
    body,
    ...(key ? { idempotencyKey: key } : {}),
  });
  if (response.status >= 300 && response.status < 400) {
    await response.body?.cancel();
    return failure(502, 'bad_gateway');
  }
  response.headers.set('Cache-Control', 'private, no-store');
  if (path.endsWith('/export') && response.ok) {
    const id = path.split('/')[1];
    const format = url.searchParams.get('format') ?? 'json';
    response.headers.set('Content-Disposition', `attachment; filename="task-${id}.${format}"`);
  }
  return response;
}

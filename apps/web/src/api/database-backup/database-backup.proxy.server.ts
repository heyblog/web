import { loadWebServerConfig, type WebServerConfig } from '../../config.server.ts';
import { requestAuthAPI } from '../auth/auth.server.ts';
import { forwardClientAddress } from '../transport/client-ip.server.ts';

import { backupBodyLimit, backupTimeoutMs, isRecord } from './database-backup.types.ts';

interface ProxyDependencies {
  readonly configuration: WebServerConfig;
  readonly authenticate: typeof requestAuthAPI;
  readonly fetcher: typeof fetch;
}
class BackupBodyLimitError extends Error {
  readonly name = 'BackupBodyLimitError';
}
function failure(status: number, code: string): Response {
  return Response.json(
    { status, code, detail: '数据操作无法完成' },
    { status, headers: { 'Cache-Control': 'private, no-store' } },
  );
}

export async function forwardDatabaseBackup(
  request: Request,
  action: string,
  dependencies?: ProxyDependencies,
): Promise<Response> {
  if (!['export', 'inspect', 'restore'].includes(action)) return failure(404, 'not_found');
  if (request.method !== 'POST') return failure(405, 'method_not_allowed');
  const url = new URL(request.url);
  if (url.search !== '') return failure(400, 'bad_request');
  if (
    request.headers.get('Origin') !== url.origin ||
    request.headers.get('Sec-Fetch-Site') !== 'same-origin'
  )
    return failure(403, 'forbidden');
  if (!request.headers.get('cookie')) return failure(401, 'unauthorized');
  const length = request.headers.get('content-length');
  if (length !== null && (!/^\d+$/u.test(length) || Number(length) > backupBodyLimit))
    return failure(413, 'request_too_large');
  if (
    action !== 'export' &&
    !/^multipart\/form-data\s*;/iu.test(request.headers.get('content-type') ?? '')
  )
    return failure(415, 'unsupported_media_type');
  const configuration = dependencies?.configuration ?? loadWebServerConfig();
  const authenticate = dependencies?.authenticate ?? requestAuthAPI;
  const session = await authenticate(request, '/auth/me');
  if (!session.ok)
    return failure(
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
  } catch (error) {
    if (error instanceof Error) return failure(502, 'bad_gateway');
    throw error;
  }
  if (!isRecord(actor) || !isRecord(actor.user)) return failure(502, 'bad_gateway');
  if (actor.user.role !== 'SYS_ADMIN') return failure(403, 'forbidden');
  const headers = new Headers({
    Accept: 'application/json',
    'X-HeyBlog-Web-Token': configuration.apiWebToken,
  });
  const cookie = request.headers.get('cookie');
  if (cookie) headers.set('Cookie', cookie);
  forwardClientAddress(request, headers);
  let oversized = false;
  let received = 0;
  const body =
    action === 'export'
      ? '{}'
      : request.body?.pipeThrough(
          new TransformStream<Uint8Array, Uint8Array>({
            transform(chunk, controller) {
              received += chunk.byteLength;
              if (received > backupBodyLimit) {
                oversized = true;
                throw new BackupBodyLimitError();
              }
              controller.enqueue(chunk);
            },
          }),
        );
  headers.set(
    'Content-Type',
    action === 'export' ? 'application/json' : (request.headers.get('content-type') ?? ''),
  );
  const init = {
    method: 'POST',
    headers,
    body,
    duplex: 'half',
    redirect: 'manual' as const,
    signal: AbortSignal.any([request.signal, AbortSignal.timeout(backupTimeoutMs)]),
  };
  let upstream: Response;
  try {
    upstream = await (dependencies?.fetcher ?? fetch)(
      new URL(`/management/database-backup/${action}`, configuration.apiBaseUrl),
      init,
    );
  } catch (error) {
    if (oversized) return failure(413, 'request_too_large');
    if (error instanceof Error)
      return failure(502, action === 'restore' ? 'restore_outcome_unknown' : 'bad_gateway');
    throw error;
  }
  if (upstream.status >= 300 && upstream.status < 400) return failure(502, 'bad_gateway');
  const responseHeaders = new Headers({
    'Cache-Control': 'private, no-store',
    'X-Content-Type-Options': 'nosniff',
  });
  for (const name of ['content-type', 'retry-after']) {
    const value = upstream.headers.get(name);
    if (value) responseHeaders.set(name, value);
  }
  for (const source of [session, upstream])
    for (const value of source.headers.getSetCookie()) responseHeaders.append('Set-Cookie', value);
  if (action === 'export' && upstream.ok) {
    responseHeaders.set('Content-Disposition', 'attachment; filename="heyblog-database.json"');
    responseHeaders.set('Content-Type', 'application/json');
  }
  return new Response(upstream.body, { status: upstream.status, headers: responseHeaders });
}

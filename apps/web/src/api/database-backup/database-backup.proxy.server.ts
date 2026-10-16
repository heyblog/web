import { randomBytes } from 'node:crypto';

import { loadWebServerConfig, type WebServerConfig } from '../../config.server.ts';
import { requestAuthAPI } from '../auth/auth.server.ts';
import { forwardClientAddress } from '../transport/client-ip.server.ts';

import { requestBackup } from './database-backup.http.server.ts';
import { backupControlLimit, backupTimeoutMs, isRecord } from './database-backup.types.ts';
import { backupPartSize } from './database-backup.uploads.ts';

interface ProxyDependencies {
  readonly configuration: WebServerConfig;
  readonly authenticate: typeof requestAuthAPI;
  readonly fetcher: typeof fetch;
}
class BackupBodyLimitError extends Error {
  readonly name = 'BackupBodyLimitError';
}
const validRequestID = /^[A-Za-z0-9][A-Za-z0-9._-]{7,63}$/u;
function backupFailure(status: number, code: string, requestID: string): Response {
  return Response.json(
    { status, code, detail: '数据操作无法完成' },
    { status, headers: { 'Cache-Control': 'private, no-store', 'X-Request-ID': requestID } },
  );
}

export async function forwardDatabaseBackup(
  request: Request,
  action: string,
  dependencies?: ProxyDependencies,
): Promise<Response> {
  const candidate = request.headers.get('X-Request-ID') ?? '';
  const requestID = validRequestID.test(candidate) ? candidate : randomBytes(16).toString('hex');
  const failure = (status: number, code: string) => backupFailure(status, code, requestID);
  const backupAction = ['export', 'inspect', 'restore'].includes(action);
  const id = '[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}';
  const jobAction =
    new RegExp(`^restore-jobs/(?:current|${id})(?:/cancel)?$`, 'iu').test(action) &&
    action !== 'restore-jobs/current/cancel';
  const uploadRead =
    action === 'uploads/current' || new RegExp(`^uploads/${id}$`, 'iu').test(action);
  const uploadWrite =
    action === 'uploads' ||
    new RegExp(`^uploads/${id}/(?:complete|parts/[1-9][0-9]{0,2})$`, 'iu').test(action);
  const uploadPart = new RegExp(`^uploads/${id}/parts/[1-9][0-9]{0,2}$`, 'iu').test(action);
  const binaryPart = uploadPart && request.method === 'PUT';
  if (!backupAction && !jobAction && !uploadRead && !uploadWrite) return failure(404, 'not_found');
  const method = binaryPart
    ? 'PUT'
    : backupAction || uploadWrite || action.endsWith('/cancel')
      ? 'POST'
      : request.method === 'DELETE' && action !== 'uploads/current' && uploadRead
        ? 'DELETE'
        : 'GET';
  if (request.method !== method) return failure(405, 'method_not_allowed');
  const url = new URL(request.url);
  if (url.search !== '') return failure(400, 'bad_request');
  if (
    method === 'GET'
      ? !isPageRead(request)
      : request.headers.get('Origin') !== url.origin ||
        request.headers.get('Sec-Fetch-Site') !== 'same-origin'
  )
    return failure(403, 'forbidden');
  if (!request.headers.get('cookie')) return failure(401, 'unauthorized');
  const bodyLimit = binaryPart ? backupPartSize : backupControlLimit;
  const length = request.headers.get('content-length');
  if (length !== null && (!/^\d+$/u.test(length) || Number(length) > bodyLimit))
    return failure(413, 'request_too_large');
  if (
    binaryPart
      ? !/^application\/octet-stream(?:\s*;|$)/iu.test(request.headers.get('content-type') ?? '')
      : (action === 'inspect' || action === 'restore' || uploadWrite) &&
        !/^application\/json(?:\s*;|$)/iu.test(request.headers.get('content-type') ?? '')
  )
    return failure(415, 'unsupported_media_type');
  const configuration = dependencies?.configuration ?? loadWebServerConfig();
  const authenticate = dependencies?.authenticate ?? requestAuthAPI;
  const session = await authenticate(request, '/api/web/auth/me');
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
    'X-Request-ID': requestID,
  });
  const cookie = request.headers.get('cookie');
  if (cookie) headers.set('Cookie', cookie);
  forwardClientAddress(request, headers);
  let oversized = false;
  let received = 0;
  const hasBody = method === 'POST' || binaryPart;
  let uploadComplete = !hasBody || jobAction;
  const body =
    action === 'export'
      ? '{}'
      : jobAction || !hasBody
        ? undefined
        : request.body?.pipeThrough(
            new TransformStream<Uint8Array, Uint8Array>({
              transform(chunk, controller) {
                received += chunk.byteLength;
                if (received > bodyLimit) {
                  oversized = true;
                  throw new BackupBodyLimitError();
                }
                controller.enqueue(chunk);
              },
              flush() {
                uploadComplete = true;
              },
            }),
          );
  headers.set(
    'Content-Type',
    action === 'export' ? 'application/json' : (request.headers.get('content-type') ?? ''),
  );
  const signal = AbortSignal.any([request.signal, AbortSignal.timeout(backupTimeoutMs)]);
  const started = performance.now();
  const init = {
    method,
    headers,
    body,
    duplex: 'half',
    redirect: 'manual' as const,
    signal,
  };
  let upstream: Response;
  try {
    upstream = await (dependencies?.fetcher ?? requestBackup)(
      new URL(`/api/web/management/database-backup/${action}`, configuration.apiBaseUrl),
      init,
    );
  } catch (error) {
    if (oversized) return failure(413, 'request_too_large');
    if (error instanceof Error) {
      const causeCode =
        'code' in error ? error.code : isRecord(error.cause) ? error.cause.code : undefined;
      const transportCode =
        typeof causeCode === 'string' &&
        [
          'UND_ERR_SOCKET',
          'UND_ERR_HEADERS_TIMEOUT',
          'UND_ERR_CONNECT_TIMEOUT',
          'ECONNRESET',
          'ECONNREFUSED',
          'EPIPE',
        ].includes(causeCode)
          ? causeCode
          : 'unknown';
      console.error(
        JSON.stringify({
          event: 'database_backup_proxy_failed',
          request_id: requestID,
          action,
          stage: uploadComplete ? 'response' : 'upload',
          received_bytes: received,
          reason: request.signal.aborted
            ? 'client_cancelled'
            : signal.aborted
              ? 'deadline_exceeded'
              : 'upstream_connection',
          transport_code: transportCode,
          duration_ms: Math.round(performance.now() - started),
        }),
      );
      return failure(502, action === 'restore' ? 'restore_outcome_unknown' : 'bad_gateway');
    }
    throw error;
  }
  if (upstream.status >= 300 && upstream.status < 400) {
    await upstream.body?.cancel();
    return failure(502, 'bad_gateway');
  }
  const responseHeaders = new Headers({
    'Cache-Control': 'private, no-store',
    'X-Content-Type-Options': 'nosniff',
    'X-Request-ID': requestID,
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

function isPageRead(request: Request): boolean {
  const site = request.headers.get('Sec-Fetch-Site');
  const mode = request.headers.get('Sec-Fetch-Mode');
  const destination = request.headers.get('Sec-Fetch-Dest');
  if (site !== null || mode !== null || destination !== null)
    return (
      site === 'same-origin' &&
      (mode === 'cors' || mode === 'same-origin') &&
      destination === 'empty'
    );
  const origin = new URL(request.url).origin;
  return (
    request.headers.get('Accept') === 'application/json' &&
    URL.parse(request.headers.get('Referer') ?? '')?.origin === origin &&
    (request.headers.get('Origin') === null || request.headers.get('Origin') === origin)
  );
}

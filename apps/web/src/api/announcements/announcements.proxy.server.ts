import { requestAuthAPI } from '../auth/auth.server.ts';

import { isDate, isFields, isRecord, isUUID, isVersion } from './announcements.types.ts';

const bodyLimit = 64_000;
const fieldNames = [
  'kind',
  'title',
  'bodyMarkdown',
  'priority',
  'actionType',
  'actionLabel',
  'actionPath',
  'actionExternalUrl',
  'startsAt',
  'endsAt',
];

function problem(status: number, code: string): Response {
  return Response.json(
    { status, code, detail: '请求无法处理' },
    { status, headers: { 'Cache-Control': 'private, no-store' } },
  );
}

export function announcementListQuery(url: URL): string | null {
  const allowed = ['page', 'pageSize', 'kind', 'status'];
  for (const key of url.searchParams.keys())
    if (!allowed.includes(key) || url.searchParams.getAll(key).length !== 1) return null;
  for (const key of ['page', 'pageSize']) {
    const value = url.searchParams.get(key);
    if (
      value !== null &&
      (!/^[1-9]\d*$/.test(value) ||
        !Number.isSafeInteger(Number(value)) ||
        (key === 'pageSize' && Number(value) > 100))
    )
      return null;
  }
  const kind = url.searchParams.get('kind');
  const status = url.searchParams.get('status');
  if (kind !== null && kind !== 'MAIN' && kind !== 'BANNER') return null;
  if (status !== null && status !== 'DRAFT' && status !== 'PUBLISHED' && status !== 'ARCHIVED')
    return null;
  const query = url.searchParams.toString();
  return query ? `?${query}` : '';
}

async function readBody(request: Request): Promise<unknown> {
  if (!request.body) return null;
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  try {
    for (;;) {
      const result = await reader.read();
      if (result.done) break;
      size += result.value.byteLength;
      if (size > bodyLimit) {
        await reader.cancel();
        return null;
      }
      chunks.push(result.value);
    }
  } finally {
    reader.releaseLock();
  }
  const bytes = new Uint8Array(size);
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

function validPayload(
  value: unknown,
  method: string,
  action: string | undefined,
): value is Readonly<Record<string, unknown>> {
  if (!isRecord(value)) return false;
  if (action === 'publish')
    return (
      Object.keys(value).every((key) => ['rowVersion', 'startsAt', 'endsAt'].includes(key)) &&
      isVersion(value.rowVersion) &&
      (value.startsAt === null || isDate(value.startsAt)) &&
      (value.endsAt === null || isDate(value.endsAt))
    );
  if (action === 'archive' || method === 'DELETE')
    return Object.keys(value).length === 1 && isVersion(value.rowVersion);
  return (
    Object.keys(value).every(
      (key) => fieldNames.includes(key) || (method === 'PUT' && key === 'rowVersion'),
    ) &&
    isFields(value) &&
    (method !== 'PUT' || isVersion(value.rowVersion))
  );
}

export async function forwardAnnouncementManagement(
  request: Request,
  path = '',
  upstream: typeof requestAuthAPI = requestAuthAPI,
): Promise<Response> {
  const parts = path ? path.split('/') : [];
  const [id, action] = parts;
  if (
    parts.length > 2 ||
    (id !== undefined && !isUUID(id)) ||
    (action !== undefined && !['publish', 'archive', 'revisions'].includes(action))
  )
    return problem(404, 'not_found');
  const method = request.method;
  const accepted = !id
    ? ['GET', 'POST']
    : !action
      ? ['GET', 'PUT', 'DELETE']
      : action === 'revisions'
        ? ['GET']
        : ['POST'];
  if (!accepted.includes(method)) return problem(405, 'method_not_allowed');
  const incomingURL = new URL(request.url);
  const query =
    !id && method === 'GET'
      ? announcementListQuery(incomingURL)
      : incomingURL.search === ''
        ? ''
        : null;
  if (query === null) return problem(400, 'bad_request');
  const target: `/${string}` = `/management/announcements${path ? `/${path}` : ''}${query}`;
  let response: Response;
  if (method === 'GET') response = await upstream(request, target);
  else {
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
    const value = await readBody(request);
    if (!validPayload(value, method, action)) return problem(422, 'validation_failed');
    if (method !== 'POST' && method !== 'PUT' && method !== 'DELETE')
      return problem(405, 'method_not_allowed');
    response = await upstream(request, target, { method, body: value });
  }
  response.headers.set('Cache-Control', 'private, no-store');
  response.headers.delete('Location');
  return response;
}

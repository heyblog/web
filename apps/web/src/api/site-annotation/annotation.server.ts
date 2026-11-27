import { requestAuthAPI } from '../auth/auth.server.ts';

import { annotationAPIPath, annotationBodyLimit, annotationTarget } from './annotation-contract.ts';
import { parseAnnotationPatch } from './annotation-patch.ts';

function problem(status: number, code: string): Response {
  return Response.json(
    { status, code },
    { status, headers: { 'Cache-Control': 'private, no-store' } },
  );
}

function isPageRequest(request: Request): boolean {
  const origin = new URL(request.url).origin;
  const requestOrigin = request.headers.get('Origin');
  if (requestOrigin !== null && requestOrigin !== origin) return false;
  if (request.method === 'PATCH' && requestOrigin !== origin) return false;

  const site = request.headers.get('Sec-Fetch-Site');
  const mode = request.headers.get('Sec-Fetch-Mode');
  const destination = request.headers.get('Sec-Fetch-Dest');
  if (site !== null || mode !== null || destination !== null) {
    return (
      site === 'same-origin' &&
      (mode === 'cors' || mode === 'same-origin') &&
      destination === 'empty'
    );
  }
  // Fetch Metadata may be absent on plain HTTP LAN origins.
  return (
    request.headers.get('Accept') === 'application/json' &&
    URL.parse(request.headers.get('Referer') ?? '')?.origin === origin
  );
}

async function readBody(request: Request): Promise<string | null> {
  const reader = request.body?.getReader();
  if (!reader) return '';
  const decoder = new TextDecoder();
  let size = 0;
  let text = '';
  try {
    for (;;) {
      const chunk = await reader.read();
      if (chunk.done) return text + decoder.decode();
      size += chunk.value.byteLength;
      if (size > annotationBodyLimit) {
        await reader.cancel();
        return null;
      }
      text += decoder.decode(chunk.value, { stream: true });
    }
  } finally {
    reader.releaseLock();
  }
}

export async function forwardAnnotation(request: Request, path: string): Promise<Response> {
  const target = annotationTarget(path, request.method, new URL(request.url).searchParams);
  if (target === null) return problem(404, 'not_found');
  if (!isPageRequest(request)) return problem(403, 'forbidden');

  let body: Readonly<Record<string, unknown>> | undefined;
  if (request.method === 'PATCH') {
    if (request.headers.get('Content-Type')?.split(';')[0]?.trim() !== 'application/json')
      return problem(415, 'unsupported_media_type');
    try {
      const text = await readBody(request);
      if (text === null) return problem(413, 'request_too_large');
      body = { ...parseAnnotationPatch(JSON.parse(text)) };
    } catch (error) {
      if (!(error instanceof Error)) throw error;
      return problem(400, 'invalid_request');
    }
  }

  const response = await requestAuthAPI(request, `${annotationAPIPath}${target}`, {
    method: body ? 'PATCH' : 'GET',
    ...(body ? { body } : {}),
  });
  response.headers.delete('Location');
  response.headers.set('Cache-Control', 'private, no-store');
  return response;
}

export async function readAnnotation<T>(
  request: Request,
  path: string,
  parse: (value: unknown) => T,
): Promise<{ readonly data: T | null; readonly status: number; readonly headers: Headers }> {
  const target = annotationTarget(path, 'GET', new URL(request.url).searchParams);
  if (target === null) return { data: null, status: 404, headers: new Headers() };
  const response = await requestAuthAPI(request, `${annotationAPIPath}${target}`);
  if (!response.ok) return { data: null, status: response.status, headers: response.headers };
  try {
    return {
      data: parse(await response.json()),
      status: response.status,
      headers: response.headers,
    };
  } catch (error) {
    if (!(error instanceof Error)) throw error;
    return { data: null, status: 502, headers: response.headers };
  }
}

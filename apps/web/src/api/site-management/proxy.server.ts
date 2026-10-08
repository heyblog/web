import { requestAuthAPI } from '../auth/auth.server.ts';

const identifier = '[a-zA-Z0-9_-]+';
const accountRoutes: readonly [RegExp, readonly string[]][] = [
  [/^sites$/, ['GET']],
  [new RegExp(`^sites/${identifier}$`), ['GET']],
  [/^site-submissions$/, ['GET', 'POST']],
  [new RegExp(`^site-submissions/${identifier}$`), ['GET']],
  [new RegExp(`^sites/${identifier}/updates$`), ['POST']],
  [new RegExp(`^sites/${identifier}/friend-links$`), ['GET']],
  [new RegExp(`^sites/${identifier}/friend-links/${identifier}$`), ['PUT', 'DELETE']],
  [new RegExp(`^sites/${identifier}/friend-links/by-host/[a-zA-Z0-9.-]+$`), ['DELETE']],
  [new RegExp(`^sites/${identifier}/friend-link-submissions$`), ['POST']],
  [new RegExp(`^sites/${identifier}/friend-link-requests/${identifier}$`), ['DELETE']],
  [/^site-claims$/, ['GET', 'POST']],
  [new RegExp(`^site-claims/${identifier}$`), ['DELETE']],
  [new RegExp(`^site-claims/${identifier}/check$`), ['POST']],
];
const managementRoutes: readonly [RegExp, readonly string[]][] = [
  [/^site-claims$/, ['GET']],
  [new RegExp(`^site-claims/${identifier}$`), ['GET']],
  [new RegExp(`^site-claims/${identifier}/review$`), ['POST']],
  [new RegExp(`^site-ownership/${identifier}$`), ['PUT', 'DELETE']],
];

export function acceptsSiteManagementRoute(
  path: string,
  method: string,
  management = false,
): boolean {
  return (management ? managementRoutes : accountRoutes).some(
    ([pattern, methods]) => pattern.test(path) && methods.includes(method),
  );
}

function problem(status: number, code: string): Response {
  return Response.json(
    { status, code },
    { status, headers: { 'Cache-Control': 'private, no-store' } },
  );
}

export async function forwardSiteManagement(
  request: Request,
  path: string,
  management = false,
): Promise<Response> {
  if (!acceptsSiteManagementRoute(path, request.method, management))
    return problem(404, 'not_found');
  let body: Readonly<Record<string, unknown>> | undefined;
  if (request.method !== 'GET') {
    const origin = request.headers.get('Origin');
    if (
      request.headers.get('Sec-Fetch-Site') !== 'same-origin' ||
      (origin && origin !== new URL(request.url).origin)
    )
      return problem(403, 'forbidden');
    if (request.headers.get('Content-Type')?.split(';')[0]?.trim() !== 'application/json')
      return problem(415, 'unsupported_media_type');
    const reader = request.body?.getReader();
    let text = '';
    if (reader) {
      const decoder = new TextDecoder();
      let bytes = 0;
      try {
        while (true) {
          const chunk = await reader.read();
          if (chunk.done) break;
          bytes += chunk.value.byteLength;
          if (bytes > 64 * 1024) {
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
    }
    try {
      const parsed: unknown = text ? JSON.parse(text) : {};
      if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed))
        return problem(400, 'invalid_request');
      body = Object.fromEntries(Object.entries(parsed));
    } catch {
      return problem(400, 'invalid_request');
    }
  }
  const query = new URL(request.url).searchParams;
  const allowedQuery = new URLSearchParams();
  for (const name of ['page', 'page_size']) {
    const value = query.get(name);
    if (value && /^\d{1,4}$/.test(value)) allowedQuery.set(name, value);
  }
  const suffix = allowedQuery.size ? `?${allowedQuery}` : '';
  const upstream = await requestAuthAPI(
    request,
    `/${management ? 'management' : 'account'}/${path}${suffix}`,
    {
      method:
        request.method === 'GET'
          ? 'GET'
          : request.method === 'POST'
            ? 'POST'
            : request.method === 'PUT'
              ? 'PUT'
              : 'DELETE',
      ...(body ? { body } : {}),
    },
  );
  upstream.headers.delete('Location');
  upstream.headers.set('Cache-Control', 'private, no-store');
  return upstream;
}

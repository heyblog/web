import { randomBytes } from 'node:crypto';

import type { APIRoute } from 'astro';

import { loadWebServerConfig, type WebServerConfig } from '../../config.server.ts';

export const apiWebTokenHeader = 'X-HeyBlog-Web-Token';

type ForwardableRequestHeader =
  'accept' | 'accept-language' | 'authorization' | 'content-type' | 'cookie';

type ForwardableResponseHeader =
  'content-type' | 'etag' | 'last-modified' | 'location' | 'www-authenticate';

interface EndpointPolicyBase {
  method: 'GET';
  upstreamPath: `/${string}`;
  queryParameters?: readonly string[];
  requestHeaders?: readonly ForwardableRequestHeader[];
  responseHeaders: readonly ForwardableResponseHeader[];
  forwardSetCookie?: boolean;
  timeoutMs?: number;
}

export interface PublicCorsPolicy {
  allowOrigins: '*' | readonly string[];
  allowHeaders?: readonly string[];
}

export type ApiEndpointPolicy = EndpointPolicyBase &
  (
    | { audience: 'web-only' }
    | {
        audience: 'public';
        cors: PublicCorsPolicy;
      }
  );

export interface ApiEndpointDependencies {
  fetch?: typeof fetch;
  loadConfig?: () => WebServerConfig;
}

export interface ApiEndpointHandlers {
  GET: APIRoute;
  OPTIONS: APIRoute;
}

const validRequestID = /^[A-Za-z0-9][A-Za-z0-9._-]{7,63}$/;

export function createApiEndpoint(
  policy: ApiEndpointPolicy,
  dependencies: ApiEndpointDependencies = {},
): ApiEndpointHandlers {
  validatePolicy(policy);
  return {
    GET: ({ request }) => handleApiRequest(request, policy, dependencies),
    OPTIONS: ({ request }) => handlePreflight(request, policy),
  };
}

export async function handleApiRequest(
  request: Request,
  policy: ApiEndpointPolicy,
  dependencies: ApiEndpointDependencies = {},
): Promise<Response> {
  const requestID = resolveRequestID(request);
  if (request.method !== policy.method) {
    return problemResponse(
      request,
      requestID,
      405,
      'method_not_allowed',
      'the method is not allowed',
    );
  }

  if (policy.audience === 'web-only' && !isWebPageFetch(request)) {
    return problemResponse(request, requestID, 403, 'forbidden', 'the request is not allowed');
  }

  const corsHeaders = resolveCorsHeaders(request, policy);
  if (corsHeaders === null) {
    return problemResponse(
      request,
      requestID,
      403,
      'forbidden',
      'the request origin is not allowed',
    );
  }

  const upstreamURL = resolveUpstreamURL(request, policy, requestID);
  if (upstreamURL instanceof Response) {
    return upstreamURL;
  }

  let configuration: WebServerConfig;
  try {
    configuration = (dependencies.loadConfig ?? loadWebServerConfig)();
  } catch {
    return problemResponse(
      request,
      requestID,
      500,
      'internal_error',
      'the web service is not configured',
    );
  }

  const upstreamHeaders = new Headers({
    Accept: 'application/json',
    [apiWebTokenHeader]: configuration.apiWebToken,
    'X-Request-ID': requestID,
  });
  for (const header of policy.requestHeaders ?? []) {
    const value = request.headers.get(header);
    if (value !== null) {
      upstreamHeaders.set(header, value);
    }
  }

  let upstreamResponse: Response;
  try {
    upstreamResponse = await (dependencies.fetch ?? fetch)(
      new URL(upstreamURL.pathname + upstreamURL.search, configuration.apiBaseUrl),
      {
        method: policy.method,
        headers: upstreamHeaders,
        redirect: 'manual',
        signal: AbortSignal.timeout(policy.timeoutMs ?? 5_000),
      },
    );
  } catch {
    return problemResponse(
      request,
      requestID,
      502,
      'bad_gateway',
      'the upstream service is unavailable',
    );
  }

  const responseHeaders = new Headers(corsHeaders);
  responseHeaders.set('Cache-Control', 'no-store');
  responseHeaders.set(
    'X-Request-ID',
    validRequestID.test(upstreamResponse.headers.get('X-Request-ID') ?? '')
      ? (upstreamResponse.headers.get('X-Request-ID') as string)
      : requestID,
  );
  for (const header of policy.responseHeaders) {
    const value = upstreamResponse.headers.get(header);
    if (value !== null) {
      responseHeaders.set(header, value);
    }
  }
  if (policy.forwardSetCookie) {
    for (const cookie of upstreamResponse.headers.getSetCookie()) {
      responseHeaders.append('Set-Cookie', cookie);
    }
  }

  return new Response(upstreamResponse.body, {
    status: upstreamResponse.status,
    headers: responseHeaders,
  });
}

function handlePreflight(request: Request, policy: ApiEndpointPolicy): Response {
  const requestID = resolveRequestID(request);
  if (policy.audience !== 'public') {
    return problemResponse(request, requestID, 403, 'forbidden', 'the request is not allowed');
  }
  if (request.headers.get('Access-Control-Request-Method') !== policy.method) {
    return problemResponse(
      request,
      requestID,
      403,
      'forbidden',
      'the requested method is not allowed',
    );
  }

  const corsHeaders = resolveCorsHeaders(request, policy);
  if (corsHeaders === null || !requestedHeadersAllowed(request, policy.cors)) {
    return problemResponse(
      request,
      requestID,
      403,
      'forbidden',
      'the preflight request is not allowed',
    );
  }
  corsHeaders.set('Access-Control-Allow-Methods', policy.method);
  corsHeaders.set('Access-Control-Allow-Headers', (policy.cors.allowHeaders ?? []).join(', '));
  corsHeaders.set('Access-Control-Max-Age', '600');
  corsHeaders.set('Cache-Control', 'no-store');
  corsHeaders.set('Vary', appendVary(corsHeaders.get('Vary'), 'Access-Control-Request-Method'));
  corsHeaders.set('Vary', appendVary(corsHeaders.get('Vary'), 'Access-Control-Request-Headers'));
  return new Response(null, { status: 204, headers: corsHeaders });
}

function isWebPageFetch(request: Request): boolean {
  const mode = request.headers.get('Sec-Fetch-Mode');
  return (
    request.headers.get('Sec-Fetch-Site') === 'same-origin' &&
    (mode === 'cors' || mode === 'same-origin') &&
    request.headers.get('Sec-Fetch-Dest') === 'empty'
  );
}

function resolveCorsHeaders(request: Request, policy: ApiEndpointPolicy): Headers | null {
  const headers = new Headers();
  if (policy.audience !== 'public') {
    return headers;
  }

  const origin = request.headers.get('Origin');
  if (origin === null) {
    return headers;
  }
  if (policy.cors.allowOrigins === '*') {
    headers.set('Access-Control-Allow-Origin', '*');
    return headers;
  }
  if (!policy.cors.allowOrigins.includes(origin)) {
    return null;
  }
  headers.set('Access-Control-Allow-Origin', origin);
  headers.set('Vary', 'Origin');
  return headers;
}

function requestedHeadersAllowed(request: Request, policy: PublicCorsPolicy): boolean {
  const allowed = new Set((policy.allowHeaders ?? []).map((header) => header.toLowerCase()));
  return (
    request.headers
      .get('Access-Control-Request-Headers')
      ?.split(',')
      .map((header) => header.trim().toLowerCase())
      .filter(Boolean)
      .every((header) => allowed.has(header)) ?? true
  );
}

function resolveUpstreamURL(
  request: Request,
  policy: ApiEndpointPolicy,
  requestID: string,
): URL | Response {
  const incomingURL = new URL(request.url);
  const allowedParameters = new Set(policy.queryParameters ?? []);
  for (const parameter of incomingURL.searchParams.keys()) {
    if (!allowedParameters.has(parameter)) {
      return problemResponse(
        request,
        requestID,
        400,
        'bad_request',
        'the query parameter is not allowed',
      );
    }
  }

  const upstreamURL = new URL(policy.upstreamPath, 'http://upstream.invalid');
  for (const parameter of allowedParameters) {
    for (const value of incomingURL.searchParams.getAll(parameter)) {
      upstreamURL.searchParams.append(parameter, value);
    }
  }
  return upstreamURL;
}

function validatePolicy(policy: ApiEndpointPolicy): void {
  if (
    !policy.upstreamPath.startsWith('/') ||
    policy.upstreamPath.startsWith('//') ||
    policy.upstreamPath.includes('?') ||
    policy.upstreamPath.includes('#')
  ) {
    throw new Error('upstreamPath must be an absolute path without query or fragment');
  }
  if ((policy.timeoutMs ?? 5_000) <= 0) {
    throw new Error('timeoutMs must be positive');
  }
}

function resolveRequestID(request: Request): string {
  const candidate = request.headers.get('X-Request-ID') ?? '';
  return validRequestID.test(candidate) ? candidate : randomBytes(16).toString('hex');
}

function problemResponse(
  request: Request,
  requestID: string,
  status: number,
  code: string,
  detail: string,
): Response {
  const title =
    status === 400
      ? 'Bad Request'
      : status === 403
        ? 'Forbidden'
        : status === 405
          ? 'Method Not Allowed'
          : status === 502
            ? 'Bad Gateway'
            : 'Internal Server Error';
  return Response.json(
    {
      type: `urn:heyblog:problem:${code}`,
      title,
      status,
      detail,
      instance: new URL(request.url).pathname,
      code,
      request_id: requestID,
    },
    {
      status,
      headers: {
        'Cache-Control': 'no-store',
        'Content-Type': 'application/problem+json',
        'X-Request-ID': requestID,
      },
    },
  );
}

function appendVary(current: string | null, value: string): string {
  return current ? `${current}, ${value}` : value;
}

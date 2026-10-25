import { copySetCookie, readProblemCode, requestAuthAPI } from '../../api/auth/auth.server.ts';
import {
  oauthAuthorizationURL,
  type OAuthCompletion,
  type OAuthProvider,
  parseOAuthCompletion,
} from '../../api/auth/oauth.types.ts';

import { safeNext } from './auth-routing.ts';

export async function startOAuth(request: Request, provider: OAuthProvider): Promise<Response> {
  const incoming = new URL(request.url);
  const response = await requestAuthAPI(
    request,
    `/api/web/auth/${provider}/start${incoming.search}`,
  );
  const headers = new Headers({ 'Cache-Control': 'private, no-store' });
  const requestID = response.headers.get('X-Request-ID');
  if (requestID) headers.set('X-Request-ID', requestID);
  copySetCookie(response, headers);
  const wantsJSON = request.headers.get('accept')?.includes('application/json');
  if (!response.ok) {
    if (!wantsJSON) {
      const next =
        incoming.searchParams.get('intent') === 'login' || !incoming.searchParams.has('intent')
          ? '/login'
          : safeNext(incoming.searchParams.get('next'));
      const target = new URL(next, incoming.origin);
      target.searchParams.set('error', await readProblemCode(response));
      headers.set('Location', target.href);
      return new Response(null, { status: 302, headers });
    }
    return new Response(response.body, {
      status: response.status,
      headers: new Headers([...headers, ['Content-Type', 'application/problem+json']]),
    });
  }
  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    return oauthFailure(headers);
  }
  if (
    typeof payload !== 'object' ||
    payload === null ||
    !('authorization_url' in payload) ||
    !('state' in payload)
  )
    return oauthFailure(headers);
  const target = oauthAuthorizationURL(provider, payload.authorization_url);
  if (
    !target ||
    typeof payload.state !== 'string' ||
    !payload.state ||
    new URL(target).searchParams.get('state') !== payload.state
  )
    return oauthFailure(headers);
  if (wantsJSON)
    return Response.json({ authorization_url: target, state: payload.state }, { headers });
  headers.set('Location', target);
  return new Response(null, { status: 302, headers });
}

function oauthFailure(headers: Headers) {
  return Response.json({ code: 'request_failed', status: 502 }, { status: 502, headers });
}

export async function completeOAuth(
  request: Request,
  provider: OAuthProvider,
): Promise<
  | {
      readonly headers: Headers;
      readonly result: OAuthCompletion;
    }
  | Response
> {
  const incoming = new URL(request.url);
  const response = await requestAuthAPI(
    request,
    `/api/web/auth/${provider}/callback${incoming.search}`,
  );
  const headers = new Headers({
    'Cache-Control': 'private, no-store',
    'Referrer-Policy': 'no-referrer',
    'X-Robots-Tag': 'noindex, nofollow',
  });
  const requestID = response.headers.get('X-Request-ID');
  if (requestID) headers.set('X-Request-ID', requestID);
  copySetCookie(response, headers);
  let result: OAuthCompletion | null = null;
  if (response.ok) {
    try {
      result = parseOAuthCompletion(await response.json());
    } catch {
      result = null;
    }
  }
  if (
    !result ||
    result.provider !== provider ||
    result.state !== incoming.searchParams.get('state')
  ) {
    const code = response.ok ? 'request_failed' : await readProblemCode(response);
    result = {
      provider,
      state: incoming.searchParams.get('state') ?? '',
      intent: 'login',
      mode: 'popup',
      next: '/login',
      outcome: 'error',
      error_code: code,
    };
  }
  const next = safeNext(result.next);
  if (result.mode === 'redirect') {
    const target = new URL(next, incoming.origin);
    if (result.outcome === 'error')
      target.searchParams.set('error', result.error_code ?? 'request_failed');
    headers.set('Location', target.href);
    return new Response(null, { status: 303, headers });
  }
  return { headers, result: { ...result, next } };
}

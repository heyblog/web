import { parseRequestID } from '../transport/request-id.ts';

import { oauthAuthorizationURL, type OAuthRequest, oauthStartPath } from './oauth.types.ts';

export class OAuthProblem extends Error {
  readonly code: string;
  readonly requestID: string | undefined;
  constructor(code: string, requestID?: string) {
    super(code);
    this.name = 'OAuthProblem';
    this.code = code;
    this.requestID = requestID;
  }
}

export async function requestOAuthStart(input: OAuthRequest, signal: AbortSignal) {
  let response: Response;
  const path = oauthStartPath(input, 'popup');
  try {
    response = await fetch(path, {
      credentials: 'same-origin',
      headers: { Accept: 'application/json' },
      cache: 'no-store',
      signal: AbortSignal.any([signal, AbortSignal.timeout(10_000)]),
    });
  } catch {
    throw new OAuthProblem(signal.aborted ? 'oauth_cancelled' : 'bad_gateway');
  }
  const requestID = parseRequestID(response.headers.get('X-Request-ID')) ?? undefined;
  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    throw new OAuthProblem('request_failed', requestID);
  }
  if (typeof payload !== 'object' || payload === null || Array.isArray(payload))
    throw new OAuthProblem('request_failed', requestID);
  const item: Record<string, unknown> = Object.fromEntries(Object.entries(payload));
  if (!response.ok)
    throw new OAuthProblem(typeof item.code === 'string' ? item.code : 'request_failed', requestID);
  const authorizationURL = oauthAuthorizationURL(input.provider, item.authorization_url);
  if (
    !authorizationURL ||
    typeof item.state !== 'string' ||
    !item.state ||
    new URL(authorizationURL).searchParams.get('state') !== item.state
  )
    throw new OAuthProblem('request_failed', requestID);
  return { authorizationURL, state: item.state, requestID };
}

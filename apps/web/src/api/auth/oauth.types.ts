export const oauthProviders = ['github', 'blogsclub'] as const;
export type OAuthProvider = (typeof oauthProviders)[number];
export type OAuthIdentityProvider = 'GITHUB' | 'BLOGSCLUB';
export type OAuthIntent = 'login' | 'bind' | 'site-claim';

export interface OAuthRequest {
  readonly provider: OAuthProvider;
  readonly intent: OAuthIntent;
  readonly next: string;
  readonly claimId?: string;
}

export interface OAuthCompletion {
  readonly provider: OAuthProvider;
  readonly state: string;
  readonly intent: OAuthIntent;
  readonly mode: 'popup' | 'redirect';
  readonly next: string;
  readonly outcome: 'success' | 'error';
  readonly error_code?: string;
  readonly claim_id?: string;
}

export function isOAuthProvider(value: unknown): value is OAuthProvider {
  return oauthProviders.some((provider) => provider === value);
}

export function isOAuthIdentityProvider(value: unknown): value is OAuthIdentityProvider {
  return value === 'GITHUB' || value === 'BLOGSCLUB';
}

export function parseOAuthCompletion(value: unknown): OAuthCompletion | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const item: Record<string, unknown> = Object.fromEntries(Object.entries(value));
  if (
    !isOAuthProvider(item.provider) ||
    typeof item.state !== 'string' ||
    !item.state ||
    (item.intent !== 'login' && item.intent !== 'bind' && item.intent !== 'site-claim') ||
    (item.mode !== 'popup' && item.mode !== 'redirect') ||
    typeof item.next !== 'string' ||
    (item.outcome !== 'success' && item.outcome !== 'error') ||
    (item.outcome === 'error' && typeof item.error_code !== 'string') ||
    (item.claim_id !== undefined && typeof item.claim_id !== 'string')
  )
    return null;
  return {
    provider: item.provider,
    state: item.state,
    intent: item.intent,
    mode: item.mode,
    next: item.next,
    outcome: item.outcome,
    ...(typeof item.error_code === 'string' ? { error_code: item.error_code } : {}),
    ...(typeof item.claim_id === 'string' ? { claim_id: item.claim_id } : {}),
  };
}

export function oauthStartPath(input: OAuthRequest, mode: 'popup' | 'redirect'): string {
  const query = new URLSearchParams({ intent: input.intent, next: input.next, mode });
  if (input.claimId) query.set('claim_id', input.claimId);
  return `/api/web/auth/${input.provider}/start?${query}`;
}

export function oauthAuthorizationURL(provider: OAuthProvider, value: unknown): string | null {
  if (typeof value !== 'string') return null;
  const target = URL.parse(value);
  if (!target || target.username || target.password) return null;
  const allowed =
    provider === 'github'
      ? ['https://github.com', '/login/oauth/authorize']
      : ['https://www.blogsclub.org', '/Oauth/Login'];
  return target.origin === allowed[0] && target.pathname === allowed[1] ? target.href : null;
}

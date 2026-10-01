import { requestAuthAPI } from '../../api/auth/auth.server.ts';
import type { SessionUser } from '../../api/auth/auth.types.ts';

export async function readSessionUser(request: Request): Promise<SessionUser | null> {
  if (!request.headers.get('cookie')) return null;
  const response = await requestAuthAPI(request, '/auth/me');
  if (response.status === 401) return null;
  if (!response.ok) throw new Error('failed to resolve session user');
  const payload = (await response.json()) as { readonly user: SessionUser };
  return payload.user;
}

export function safeNext(value: FormDataEntryValue | string | null | undefined): string {
  if (typeof value !== 'string') return '/dashboard';
  const path = value.trim();
  return path.startsWith('/') && !path.startsWith('//') ? path : '/dashboard';
}

export function pageLocation(request: Request, path: string): string {
  return new URL(path, request.url).toString();
}

export type OAuthRoute = 'github/start' | 'github/callback';

export function resolveOAuthLocation(
  request: Request,
  route: OAuthRoute,
  location: string | null,
): string | null {
  if (!location) return null;
  let target: URL;
  try {
    target = new URL(location, request.url);
  } catch {
    return null;
  }
  if (route === 'github/start') {
    return target.origin === 'https://github.com' ? target.toString() : null;
  }
  return new URL(target.pathname + target.search, request.url).toString();
}

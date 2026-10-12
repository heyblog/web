import { parseSessionResponse } from '../../api/auth/auth.responses.ts';
import { requestAuthAPI } from '../../api/auth/auth.server.ts';
import type { SessionUser } from '../../api/auth/auth.types.ts';

export class SessionUnavailableError extends Error {
  readonly name = 'SessionUnavailableError';
  readonly status: number;

  constructor(status: number) {
    super('session service is temporarily unavailable');
    this.status = status;
  }
}

export async function readSessionUser(request: Request): Promise<SessionUser | null> {
  if (!request.headers.get('cookie')) return null;
  const response = await requestAuthAPI(request, '/api/web/auth/me');
  if (response.status === 401) return null;
  if (!response.ok) throw new SessionUnavailableError(response.status);
  const user = parseSessionResponse(await response.json());
  if (!user) throw new Error('invalid session response');
  return user;
}

export function safeNext(value: FormDataEntryValue | string | null | undefined): string {
  if (typeof value !== 'string') return '/dashboard';
  const path = value.trim();
  if (
    !path.startsWith('/') ||
    path.startsWith('//') ||
    path.includes('\\') ||
    [...path].some((character) => character.charCodeAt(0) < 32 || character.charCodeAt(0) === 127)
  )
    return '/dashboard';
  const target = new URL(path, 'https://local.invalid');
  return target.origin === 'https://local.invalid'
    ? target.pathname + target.search + target.hash
    : '/dashboard';
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

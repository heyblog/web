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

export { safeNext } from './auth-routing.ts';

export function pageLocation(request: Request, path: string): string {
  return new URL(path, request.url).toString();
}

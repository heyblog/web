import { isRecord, parseUserResponse } from './managed-users.responses.ts';
import type { AuthorizationChange, AuthorizationResult } from './managed-users.types.ts';

export async function saveAuthorization(
  userId: string,
  change: AuthorizationChange,
  fetcher: typeof fetch = fetch,
): Promise<AuthorizationResult> {
  const body = new URLSearchParams();
  if (change.kind === 'role') body.set('role', change.role);
  else change.permissions.forEach((permission) => body.append('permissions', permission));
  let response: Response;
  try {
    response = await fetcher(`/management/users/${encodeURIComponent(userId)}/${change.kind}`, {
      method: 'POST',
      headers: { Accept: 'application/json' },
      body,
      credentials: 'same-origin',
      cache: 'no-store',
      redirect: 'error',
      signal: AbortSignal.timeout(15_000),
    });
  } catch (error) {
    if (!(error instanceof Error)) throw error;
    return { ok: false, code: 'network_error', status: 0 };
  }
  let payload: unknown;
  try {
    payload = await response.json();
  } catch (error) {
    if (!(error instanceof Error)) throw error;
    return {
      ok: false,
      code: response.ok ? 'invalid_response' : 'request_failed',
      status: response.status,
    };
  }
  if (!response.ok)
    return {
      ok: false,
      status: response.status,
      code: isRecord(payload) && typeof payload.code === 'string' ? payload.code : 'request_failed',
    };
  const user = parseUserResponse(payload);
  return user && user.id === userId
    ? { ok: true, user }
    : { ok: false, code: 'invalid_response', status: 502 };
}

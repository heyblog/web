import { isRecord, parseUserResponse } from './managed-users.responses.ts';
import type {
  AuthorizationChange,
  AuthorizationResult,
  ManagedUserMutation,
  ManagedUserResult,
} from './managed-users.types.ts';

export async function saveAuthorization(
  userId: string,
  change: AuthorizationChange,
  fetcher: typeof fetch = fetch,
): Promise<AuthorizationResult> {
  const body = new URLSearchParams({ updated_at: change.updated_at });
  if (change.kind === 'role') body.set('role', change.role);
  else change.permissions.forEach((permission) => body.append('permissions', permission));
  let response: Response;
  try {
    response = await fetcher(
      `/api/web/management/users/${encodeURIComponent(userId)}/${change.kind}`,
      {
        method: 'POST',
        headers: { Accept: 'application/json' },
        body,
        credentials: 'same-origin',
        cache: 'no-store',
        redirect: 'error',
        signal: AbortSignal.timeout(15_000),
      },
    );
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

export async function mutateManagedUser(
  userId: string | null,
  change: ManagedUserMutation,
  fetcher: typeof fetch = fetch,
): Promise<ManagedUserResult> {
  const { operation, ...body } = change;
  try {
    const response = await fetcher(
      operation === 'create'
        ? '/api/web/management/users'
        : `/api/web/management/users/${encodeURIComponent(userId ?? '')}/${operation}`,
      {
        method: operation === 'profile' || operation === 'status' ? 'PATCH' : 'POST',
        headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        credentials: 'same-origin',
        cache: 'no-store',
        redirect: 'error',
        signal: AbortSignal.timeout(15_000),
      },
    );
    const payload: unknown = await response.json().catch(() => null);
    if (!response.ok)
      return {
        ok: false,
        status: response.status,
        code:
          isRecord(payload) && typeof payload.code === 'string' ? payload.code : 'request_failed',
      };
    const user = parseUserResponse(payload);
    if (!user || (userId && user.id !== userId))
      return { ok: false, code: 'invalid_response', status: 502 };
    const mailOperation = ['create', 'password-reset', 'verification'].includes(operation);
    if (mailOperation && (!isRecord(payload) || typeof payload.mail_sent !== 'boolean'))
      return { ok: false, code: 'invalid_response', status: 502 };
    return {
      ok: true,
      user,
      ...(mailOperation && isRecord(payload) && typeof payload.mail_sent === 'boolean'
        ? { mailSent: payload.mail_sent }
        : {}),
    };
  } catch {
    return { ok: false, code: 'network_error', status: 0 };
  }
}

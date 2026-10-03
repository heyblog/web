import { requestAuthAPI } from '../auth/auth.server.ts';

import {
  isPermission,
  isRecord,
  parseUserResponse,
  parseUsersResponse,
} from './managed-users.responses.ts';
import type { AuthorizationOperation, ManagedUser } from './managed-users.types.ts';

type ReadResult<T> =
  | { readonly ok: true; readonly value: T; readonly headers: Headers }
  | {
      readonly ok: false;
      readonly status: number;
      readonly code: string;
      readonly headers: Headers;
    };

async function read<T>(
  response: Response,
  parser: (value: unknown) => T | null,
): Promise<ReadResult<T>> {
  let payload: unknown;
  try {
    payload = await response.json();
  } catch (error) {
    if (!(error instanceof Error)) throw error;
    payload = null;
  }
  if (!response.ok)
    return {
      ok: false,
      status: response.status,
      code: isRecord(payload) && typeof payload.code === 'string' ? payload.code : 'request_failed',
      headers: response.headers,
    };
  const value = parser(payload);
  return value === null
    ? { ok: false, status: 502, code: 'invalid_response', headers: response.headers }
    : { ok: true, value, headers: response.headers };
}

export async function readManagementActor(request: Request): Promise<ReadResult<ManagedUser>> {
  return read(await requestAuthAPI(request, '/auth/me'), parseUserResponse);
}

export async function readManagedUsers(
  request: Request,
): Promise<ReadResult<readonly ManagedUser[]>> {
  return read(await requestAuthAPI(request, '/management/users'), parseUsersResponse);
}

export async function updateManagedUser(
  request: Request,
  target: { readonly id: string | undefined; readonly operation: AuthorizationOperation },
  form: FormData,
): Promise<ReadResult<ManagedUser>> {
  const invalid = {
    ok: false,
    status: 422,
    code: 'validation_failed',
    headers: new Headers(),
  } as const;
  if (!target.id || !/^[a-zA-Z0-9][a-zA-Z0-9-]*$/u.test(target.id)) return invalid;
  const role = form.get('role');
  const permissions = form.getAll('permissions');
  if (target.operation === 'role' && role !== 'USER' && role !== 'ADMIN') return invalid;
  if (
    target.operation === 'permissions' &&
    (!permissions.every(isPermission) || new Set(permissions).size !== permissions.length)
  )
    return invalid;
  const response = await requestAuthAPI(
    request,
    `/management/users/${target.id}/${target.operation}`,
    {
      method: 'PATCH',
      body: target.operation === 'role' ? { role } : { permissions },
    },
  );
  return read(response, (payload) => {
    const user = parseUserResponse(payload);
    return user?.id === target.id ? user : null;
  });
}

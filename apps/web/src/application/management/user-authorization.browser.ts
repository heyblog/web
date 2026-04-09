import type { ManagementPermissionKey, SessionUser } from '@/application/auth/auth.guard';

export interface ManagedUserSnapshot extends SessionUser {
  createdTime: string | null;
  lastLoginTime: string | null;
}

export type UserAuthorizationActionCode =
  | 'unauthorized'
  | 'invalid_role_request'
  | 'invalid_permissions_request'
  | 'role_update_failed'
  | 'permission_update_failed'
  | 'request_failed';

export interface UserAuthorizationActionResult {
  ok: boolean;
  status: string | null;
  target: string | null;
  code: UserAuthorizationActionCode;
  message: string;
  redirect: string | null;
  data: ManagedUserSnapshot | null;
}

type RoleIntent = 'grant-admin' | 'revoke-admin';

const DEFAULT_FAILURE_MESSAGE = '授权请求失败，请稍后重试。';

const normalizeMessage = (value: unknown, fallback: string): string => {
  if (typeof value !== 'string') {
    return fallback;
  }

  const normalized = value.replace(/\s+/g, ' ').trim();
  return normalized || fallback;
};

const parseActionResponse = async (response: Response): Promise<UserAuthorizationActionResult> => {
  const payload = (await response.json().catch(() => null)) as {
    ok?: boolean;
    status?: string;
    target?: string;
    message?: string;
    code?: string;
    redirect?: string;
    data?: ManagedUserSnapshot;
  } | null;

  return {
    ok: payload?.ok === true,
    status: typeof payload?.status === 'string' ? payload.status : null,
    target: typeof payload?.target === 'string' ? payload.target : null,
    code:
      typeof payload?.code === 'string'
        ? (payload.code as UserAuthorizationActionCode)
        : 'request_failed',
    message: normalizeMessage(
      payload?.message,
      payload?.ok === true ? '' : DEFAULT_FAILURE_MESSAGE,
    ),
    redirect: typeof payload?.redirect === 'string' ? payload.redirect : null,
    data: payload?.data ?? null,
  };
};

const postAuthorizationAction = async (
  path: string,
  body: FormData,
): Promise<UserAuthorizationActionResult> => {
  const response = await fetch(path, {
    method: 'POST',
    headers: {
      accept: 'application/json',
    },
    body,
  });

  return parseActionResponse(response);
};

export const submitRoleAuthorizationAction = (
  userId: string,
  intent: RoleIntent,
): Promise<UserAuthorizationActionResult> => {
  const body = new FormData();
  body.set('intent', intent);

  return postAuthorizationAction(`/management/users/${userId}/role`, body);
};

export const submitPermissionAuthorizationAction = (
  userId: string,
  permissions: ManagementPermissionKey[],
): Promise<UserAuthorizationActionResult> => {
  const body = new FormData();

  for (const permission of permissions) {
    body.append('permissions', permission);
  }

  return postAuthorizationAction(`/management/users/${userId}/permissions`, body);
};

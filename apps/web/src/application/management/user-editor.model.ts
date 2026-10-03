import type { ManagementPermission, UserRole } from '../../api/auth/auth.types.ts';
import type {
  AuthorizationChange,
  AuthorizationResult,
  ManagedUser,
} from '../../api/managed-users/managed-users.types.ts';

import { authorizationError } from './users.messages.ts';
import { authorizationAccess } from './users.model.ts';

export interface UserDraft {
  readonly saved: ManagedUser;
  readonly role: UserRole;
  readonly permissions: readonly ManagementPermission[];
  readonly busy: boolean;
  readonly error: string | null;
  readonly notice: string | null;
  readonly reloadRequired: boolean;
  readonly loginRequired: boolean;
}

export function createUserDraft(user: ManagedUser): UserDraft {
  return {
    saved: user,
    role: user.role,
    permissions: [...user.permissions],
    busy: false,
    error: null,
    notice: null,
    reloadRequired: false,
    loginRequired: false,
  };
}

export function draftChanges(draft: UserDraft) {
  return {
    role: draft.role !== draft.saved.role,
    permissions:
      draft.permissions.length !== draft.saved.permissions.length ||
      draft.permissions.some((permission) => !draft.saved.permissions.includes(permission)),
  };
}

export function prepareAuthorization(
  actor: ManagedUser,
  draft: UserDraft,
  kind: AuthorizationChange['kind'],
): AuthorizationChange | null {
  if (draft.busy || draft.reloadRequired) return null;
  const dirty = draftChanges(draft);
  const access = authorizationAccess(actor, draft.saved);
  if (kind === 'role') {
    if (access.roleReason || !dirty.role || dirty.permissions || draft.role === 'SYS_ADMIN')
      return null;
    return { kind, role: draft.role };
  }
  if (access.permissionsReason || !dirty.permissions || dirty.role) return null;
  if (
    actor.role !== 'SYS_ADMIN' &&
    draft.permissions.some((permission) => !actor.permissions.includes(permission))
  )
    return null;
  return { kind, permissions: draft.permissions };
}

export function finishAuthorization(draft: UserDraft, result: AuthorizationResult): UserDraft {
  if (result.ok)
    return { ...createUserDraft(result.user), notice: '授权已更新，该用户需要重新登录。' };
  const reloadRequired =
    result.status === 401 ||
    result.status === 403 ||
    result.status === 404 ||
    result.status === 409 ||
    ['network_error', 'invalid_response', 'bad_gateway'].includes(result.code);
  return {
    ...draft,
    busy: false,
    error: authorizationError(result.code, result.status),
    notice: null,
    reloadRequired,
    loginRequired: result.status === 401,
  };
}

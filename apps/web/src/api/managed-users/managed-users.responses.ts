import {
  type ManagementPermission,
  managementPermissions,
  type UserRole,
  userRoles,
} from '../auth/auth.types.ts';
import { isOAuthIdentityProvider } from '../auth/oauth.types.ts';

import type { ManagedUser, ManagedUsersPage } from './managed-users.types.ts';

export function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function isRole(value: unknown): value is UserRole {
  return userRoles.some((role) => role === value);
}

export function isPermission(value: unknown): value is ManagementPermission {
  return managementPermissions.some((permission) => permission === value);
}

export function parseManagedUser(value: unknown): ManagedUser | null {
  if (
    !isRecord(value) ||
    typeof value.id !== 'string' ||
    !value.id ||
    typeof value.username !== 'string' ||
    typeof value.display_name !== 'string' ||
    (value.email !== null && typeof value.email !== 'string') ||
    !isRole(value.role) ||
    !Array.isArray(value.permissions) ||
    !value.permissions.every(isPermission) ||
    new Set(value.permissions).size !== value.permissions.length ||
    typeof value.active !== 'boolean' ||
    typeof value.email_verified !== 'boolean' ||
    typeof value.has_password !== 'boolean' ||
    !Array.isArray(value.oauth_providers) ||
    !value.oauth_providers.every(isOAuthIdentityProvider) ||
    new Set(value.oauth_providers).size !== value.oauth_providers.length ||
    typeof value.created_at !== 'string' ||
    typeof value.updated_at !== 'string' ||
    typeof value.access_status !== 'string' ||
    typeof value.deletion_pending !== 'boolean' ||
    (value.last_login_at !== null && typeof value.last_login_at !== 'string')
  )
    return null;
  return {
    id: value.id,
    username: value.username,
    display_name: value.display_name,
    email: value.email,
    role: value.role,
    permissions: value.permissions,
    active: value.active,
    email_verified: value.email_verified,
    has_password: value.has_password,
    oauth_providers: value.oauth_providers,
    created_at: value.created_at,
    updated_at: value.updated_at,
    last_login_at: value.last_login_at,
    access_status: value.access_status,
    deletion_pending: value.deletion_pending,
  };
}

export function parseUserResponse(value: unknown): ManagedUser | null {
  return isRecord(value) ? parseManagedUser(value.user) : null;
}

export function parseUsersResponse(value: unknown): readonly ManagedUser[] | null {
  if (!isRecord(value) || !Array.isArray(value.users)) return null;
  const users = value.users.map(parseManagedUser);
  if (users.some((user) => user === null)) return null;
  return users.filter((user) => user !== null);
}

export function parseUsersPage(value: unknown): ManagedUsersPage | null {
  const users = parseUsersResponse(value);
  if (
    !users ||
    !isRecord(value) ||
    typeof value.total !== 'number' ||
    !Number.isSafeInteger(value.total) ||
    value.total < 0 ||
    typeof value.page !== 'number' ||
    !Number.isSafeInteger(value.page) ||
    value.page < 1 ||
    typeof value.page_size !== 'number' ||
    !Number.isSafeInteger(value.page_size) ||
    value.page_size < 1 ||
    value.page_size > 100
  )
    return null;
  return { users, total: value.total, page: value.page, page_size: value.page_size };
}

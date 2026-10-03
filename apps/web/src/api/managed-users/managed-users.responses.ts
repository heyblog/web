import {
  type ManagementPermission,
  managementPermissions,
  type UserRole,
  userRoles,
} from '../auth/auth.types.ts';

import type { ManagedUser } from './managed-users.types.ts';

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
    typeof value.email_verified !== 'boolean'
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

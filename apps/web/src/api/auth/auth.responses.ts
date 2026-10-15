import {
  type ManagementPermission,
  managementPermissions,
  type SessionUser,
  type UserRole,
  userRoles,
} from './auth.types.ts';
import { isOAuthIdentityProvider } from './oauth.types.ts';

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isRole(value: unknown): value is UserRole {
  return userRoles.some((role) => role === value);
}

function isPermission(value: unknown): value is ManagementPermission {
  return managementPermissions.some((permission) => permission === value);
}

export function parseSessionResponse(payload: unknown): SessionUser | null {
  if (!isRecord(payload) || !isRecord(payload.user)) return null;
  const value = payload.user;
  if (
    typeof value.id !== 'string' ||
    !value.id ||
    typeof value.username !== 'string' ||
    typeof value.display_name !== 'string' ||
    (value.email !== null && typeof value.email !== 'string') ||
    (value.avatar_url !== null && typeof value.avatar_url !== 'string') ||
    !isRole(value.role) ||
    !Array.isArray(value.permissions) ||
    !value.permissions.every(isPermission) ||
    typeof value.active !== 'boolean' ||
    typeof value.deletion_pending !== 'boolean' ||
    (value.access_status !== 'ACTIVE' && value.access_status !== 'SUSPENDED') ||
    typeof value.email_verified !== 'boolean' ||
    typeof value.has_password !== 'boolean' ||
    !Array.isArray(value.oauth_providers) ||
    !value.oauth_providers.every(isOAuthIdentityProvider) ||
    new Set(value.oauth_providers).size !== value.oauth_providers.length ||
    typeof value.auth_version !== 'number' ||
    !Number.isSafeInteger(value.auth_version) ||
    value.auth_version < 1 ||
    typeof value.profile_confirmation_required !== 'boolean' ||
    (value.username_changes_remaining !== 0 && value.username_changes_remaining !== 1) ||
    typeof value.created_at !== 'string' ||
    typeof value.updated_at !== 'string' ||
    (value.last_login_at !== null && typeof value.last_login_at !== 'string')
  )
    return null;
  return {
    id: value.id,
    username: value.username,
    display_name: value.display_name,
    email: value.email,
    avatar_url: value.avatar_url,
    role: value.role,
    permissions: value.permissions,
    active: value.active,
    deletion_pending: value.deletion_pending,
    access_status: value.access_status,
    email_verified: value.email_verified,
    has_password: value.has_password,
    oauth_providers: value.oauth_providers,
    auth_version: value.auth_version,
    profile_confirmation_required: value.profile_confirmation_required,
    username_changes_remaining: value.username_changes_remaining,
    created_at: value.created_at,
    updated_at: value.updated_at,
    last_login_at: value.last_login_at,
  };
}

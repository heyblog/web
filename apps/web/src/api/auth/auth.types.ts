import type { OAuthIdentityProvider } from './oauth.types.ts';

export const userRoles = ['USER', 'ADMIN', 'SYS_ADMIN'] as const;
export type UserRole = (typeof userRoles)[number];

export const managementPermissions = [
  'user.manage',
  'site_audit.review',
  'feedback.review',
  'announcement.manage',
  'taxonomy.manage',
  'site.manage',
  'task.manage',
  'log.read',
] as const;
export type ManagementPermission = (typeof managementPermissions)[number];

export interface SessionUser {
  readonly id: string;
  readonly email: string | null;
  readonly username: string;
  readonly display_name: string;
  readonly avatar_url: string | null;
  readonly role: UserRole;
  readonly permissions: readonly ManagementPermission[];
  readonly active: boolean;
  readonly deletion_pending: boolean;
  readonly access_status: 'ACTIVE' | 'SUSPENDED';
  readonly profile_confirmation_required: boolean;
  readonly username_changes_remaining: number;
  readonly email_verified: boolean;
  readonly has_password: boolean;
  readonly oauth_providers: readonly OAuthIdentityProvider[];
  readonly auth_version: number;
  readonly created_at: string;
  readonly updated_at: string;
  readonly last_login_at: string | null;
}

export interface ProblemDetails {
  readonly code: string;
  readonly detail: string;
  readonly status: number;
}

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
  readonly email_verified: boolean;
  readonly has_password: boolean;
  readonly has_github: boolean;
  readonly auth_version: number;
  readonly created_at: string;
  readonly last_login_at: string | null;
}

export interface ProblemDetails {
  readonly code: string;
  readonly detail: string;
  readonly status: number;
}

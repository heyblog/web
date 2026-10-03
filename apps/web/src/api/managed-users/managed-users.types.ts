import type { ManagementPermission, SessionUser } from '../auth/auth.types.ts';

export type ManagedUser = Pick<
  SessionUser,
  | 'id'
  | 'username'
  | 'email'
  | 'display_name'
  | 'role'
  | 'permissions'
  | 'active'
  | 'email_verified'
>;

export type AuthorizationOperation = 'role' | 'permissions';

export type AuthorizationChange =
  | { readonly kind: 'role'; readonly role: 'USER' | 'ADMIN' }
  | { readonly kind: 'permissions'; readonly permissions: readonly ManagementPermission[] };

export type AuthorizationResult =
  | { readonly ok: true; readonly user: ManagedUser }
  | { readonly ok: false; readonly code: string; readonly status: number };

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
  | 'has_password'
  | 'oauth_providers'
  | 'created_at'
  | 'last_login_at'
> & {
  readonly updated_at: string;
  readonly access_status: string;
  readonly deletion_pending: boolean;
};

export interface ManagedUsersPage {
  readonly users: readonly ManagedUser[];
  readonly total: number;
  readonly page: number;
  readonly page_size: number;
}

export type AccountOperation =
  'profile' | 'status' | 'sessions/revoke' | 'password-reset' | 'verification';
export type ManagedUserMutation =
  | {
      readonly operation: 'create';
      readonly username: string;
      readonly display_name: string;
      readonly email: string;
    }
  | {
      readonly operation: 'profile';
      readonly display_name: string;
      readonly email: string;
      readonly updated_at: string;
    }
  | {
      readonly operation: 'status';
      readonly status: 'ACTIVE' | 'SUSPENDED';
      readonly updated_at: string;
    }
  | {
      readonly operation: 'sessions/revoke' | 'password-reset' | 'verification';
      readonly updated_at: string;
    };
export type ManagedUserResult =
  | { readonly ok: true; readonly user: ManagedUser; readonly mailSent?: boolean }
  | { readonly ok: false; readonly code: string; readonly status: number };

export type AuthorizationOperation = 'role' | 'permissions';

export type AuthorizationChange =
  | { readonly kind: 'role'; readonly role: 'USER' | 'ADMIN'; readonly updated_at: string }
  | {
      readonly kind: 'permissions';
      readonly permissions: readonly ManagementPermission[];
      readonly updated_at: string;
    };

export type AuthorizationResult =
  | { readonly ok: true; readonly user: ManagedUser }
  | { readonly ok: false; readonly code: string; readonly status: number };

import type { ManagementPermission, SessionUser } from '../../api/auth/auth.types.ts';

type Actor = Pick<SessionUser, 'role' | 'permissions'>;

function can(user: Actor, permission: ManagementPermission): boolean {
  return (
    user.role === 'SYS_ADMIN' || (user.role === 'ADMIN' && user.permissions.includes(permission))
  );
}

export function canAnnotateSites(user: Actor): boolean {
  return can(user, 'site_annotation.manage');
}

export function canReviewSiteAudits(user: Actor): boolean {
  return can(user, 'site_audit.review');
}

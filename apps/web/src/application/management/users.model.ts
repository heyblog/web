import type { ManagementPermission, UserRole } from '../../api/auth/auth.types.ts';
import { isRole } from '../../api/managed-users/managed-users.responses.ts';
import type { ManagedUser } from '../../api/managed-users/managed-users.types.ts';

export const roleLabels: Readonly<Record<UserRole, string>> = {
  USER: '普通用户',
  ADMIN: '管理员',
  SYS_ADMIN: '系统管理员',
};
export const permissionLabels: Readonly<Record<ManagementPermission, string>> = {
  'user.manage': '用户管理',
  'site_audit.review': '站点审核',
  'feedback.review': '反馈处理',
  'announcement.manage': '公告管理',
  'taxonomy.manage': '分类维护',
  'site.manage': '站点维护',
  'task.manage': '任务管理',
  'log.read': '日志查看',
};

export function canManageUsers(actor: ManagedUser): boolean {
  return (
    actor.role === 'SYS_ADMIN' ||
    (actor.role === 'ADMIN' && actor.permissions.includes('user.manage'))
  );
}

export function authorizationAccess(actor: ManagedUser, user: ManagedUser) {
  const self = actor.id === user.id;
  const unavailable =
    user.deletion_pending || !['ACTIVE', 'SUSPENDED'].includes(user.access_status)
      ? '此账号正在注销，暂时无法修改。'
      : null;
  const roleReason =
    unavailable ??
    (self
      ? '不能修改当前账号的角色。'
      : actor.role !== 'SYS_ADMIN'
        ? '仅系统管理员可以修改角色。'
        : null);
  const permissionsReason =
    unavailable ??
    (self
      ? '不能修改当前账号的权限。'
      : !canManageUsers(actor)
        ? '当前账号没有用户授权权限。'
        : user.role === 'SYS_ADMIN'
          ? '系统管理员拥有全部管理权限。'
          : user.role !== 'ADMIN'
            ? '设为管理员并保存角色后，可分配模块权限。'
            : actor.role !== 'SYS_ADMIN' &&
                user.permissions.some((permission) => !actor.permissions.includes(permission))
              ? '此用户的权限超出你的授权范围，请由系统管理员调整。'
              : null);
  return { roleReason, permissionsReason };
}

export function permissionSummary(user: ManagedUser): string {
  return user.role === 'SYS_ADMIN'
    ? '全部管理权限'
    : user.role === 'USER'
      ? '无管理权限'
      : `${user.permissions.length} 项模块权限`;
}

export function usersQuery(query: URLSearchParams): URLSearchParams {
  const result = new URLSearchParams();
  const q = (query.get('q') ?? '').trim().slice(0, 128);
  const role = query.get('role');
  const status = query.get('status');
  const page = Number(query.get('page') ?? 1);
  if (q) result.set('q', q);
  if (isRole(role)) result.set('role', role);
  if (status === 'ACTIVE' || status === 'SUSPENDED') result.set('status', status);
  result.set(
    'page',
    String(Number.isSafeInteger(page) && page > 0 ? Math.min(page, 1_000_000) : 1),
  );
  result.set('page_size', '20');
  return result;
}

export function accountManagementReason(actor: ManagedUser, user: ManagedUser): string | null {
  if (actor.id === user.id) return '请前往个人工作区管理当前账号。';
  if (
    user.deletion_pending ||
    (user.access_status !== 'ACTIVE' && user.access_status !== 'SUSPENDED')
  )
    return '此账号正在注销，暂时无法修改。';
  if (actor.role === 'SYS_ADMIN') return null;
  return canManageUsers(actor) && user.role === 'USER'
    ? null
    : '当前账号只能管理普通用户的账号资料。';
}

export function usersListPath(query: URLSearchParams): string {
  const accepted = new URLSearchParams();
  for (const name of ['q', 'role', 'status', 'page']) {
    const value = query.get(name);
    if (value) accepted.set(name, value);
  }
  return `/management/users${accepted.size ? `?${accepted}` : ''}`;
}

export function usersReturnPath(value: FormDataEntryValue | string | null): string {
  if (typeof value !== 'string' || !/^\/management\/users(?:\?|$)/u.test(value))
    return '/management/users';
  const url = new URL(value, 'https://local.invalid');
  return url.pathname === '/management/users'
    ? usersListPath(url.searchParams)
    : '/management/users';
}

export function userMutationNeedsReload(code: string, status: number): boolean {
  return status === 0 || status >= 500 || code === 'user_conflict' || code === 'invalid_response';
}

import type { ManagementPermission, UserRole } from '../../api/auth/auth.types.ts';
import { isRole } from '../../api/managed-users/managed-users.responses.ts';
import type { ManagedUser } from '../../api/managed-users/managed-users.types.ts';

export const roleLabels: Readonly<Record<UserRole, string>> = {
  USER: '普通用户',
  ADMIN: '管理员',
  SYS_ADMIN: '系统管理员',
};
export const permissionLabels: Readonly<Record<ManagementPermission, string>> = {
  'user.manage': '用户授权',
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
  const roleReason = self
    ? '不能修改当前账号的角色。'
    : actor.role !== 'SYS_ADMIN'
      ? '仅系统管理员可以修改角色。'
      : null;
  const permissionsReason = self
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
            : null;
  return { roleReason, permissionsReason };
}

export function permissionSummary(user: ManagedUser): string {
  return user.role === 'SYS_ADMIN'
    ? '全部管理权限'
    : user.role === 'USER'
      ? '无管理权限'
      : `${user.permissions.length} 项模块权限`;
}

export function filterUsers(users: readonly ManagedUser[], query: URLSearchParams) {
  const q = (query.get('q') ?? '').trim();
  const requestedRole = query.get('role');
  const role = isRole(requestedRole) ? requestedRole : '';
  const requestedState = query.get('state');
  const state = requestedState === 'active' || requestedState === 'inactive' ? requestedState : '';
  const needle = q.toLocaleLowerCase();
  const matches = users.filter(
    (user) =>
      (!role || user.role === role) &&
      (!state || user.active === (state === 'active')) &&
      (!needle ||
        [user.display_name, user.username, user.email ?? ''].some((value) =>
          value.toLocaleLowerCase().includes(needle),
        )),
  );
  const pages = Math.max(1, Math.ceil(matches.length / 20));
  const requestedPage = Number(query.get('page') ?? 1);
  const page = Number.isSafeInteger(requestedPage)
    ? Math.min(pages, Math.max(1, requestedPage))
    : 1;
  return {
    q,
    role,
    state,
    page,
    pages,
    total: matches.length,
    users: matches.slice((page - 1) * 20, page * 20),
  };
}

export function usersListPath(query: URLSearchParams): string {
  const accepted = new URLSearchParams();
  for (const name of ['q', 'role', 'state', 'page']) {
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

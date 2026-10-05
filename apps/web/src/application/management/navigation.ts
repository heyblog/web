import type { SessionUser } from '../../api/auth/auth.types.ts';

export interface ManagementNavigationItem {
  readonly label: string;
  readonly href: string;
  readonly description: string;
}

export function managementNavigation(
  user: Pick<SessionUser, 'role' | 'permissions'>,
): readonly ManagementNavigationItem[] {
  const items: ManagementNavigationItem[] = [];
  if (
    user.role === 'SYS_ADMIN' ||
    (user.role === 'ADMIN' && user.permissions.includes('taxonomy.manage'))
  ) {
    items.push({
      label: '标签管理',
      href: '/management/tags',
      description: '维护标签、分类与级联关系',
    });
  }
  if (
    user.role === 'SYS_ADMIN' ||
    (user.role === 'ADMIN' && user.permissions.includes('announcement.manage'))
  ) {
    items.push({
      label: '公告管理',
      href: '/management/announcements',
      description: '管理主公告与横幅公告',
    });
  }
  if (user.role === 'SYS_ADMIN' || user.permissions.includes('site_audit.review')) {
    items.push({
      label: '站点申请审核',
      href: '/management/site-submissions',
      description: '查看申请、差异与审核结果',
    });
  }
  if (user.role === 'SYS_ADMIN' || user.permissions.includes('user.manage')) {
    items.push({
      label: '用户与授权',
      href: '/management/users',
      description: '管理角色与模块权限',
    });
  }
  if (user.role === 'SYS_ADMIN') {
    items.push({
      label: '系统设置',
      href: '/management/system-settings',
      description: '设置 Slug 生成模型',
    });
    items.push({
      label: 'API 调用凭证',
      href: '/management/api-keys',
      description: '签发、轮换与撤销服务凭证',
    });
  }
  return items;
}

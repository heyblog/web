import type { ManagedUser } from '../../src/api/managed-users/managed-users.types.ts';

export const systemActor: ManagedUser = {
  id: 'system',
  username: 'system',
  email: 'system@example.test',
  display_name: '系统管理员',
  role: 'SYS_ADMIN',
  permissions: [],
  active: true,
  email_verified: true,
};
export const managedUser: ManagedUser = {
  ...systemActor,
  id: 'target',
  username: 'editor',
  display_name: '编辑',
  role: 'ADMIN',
  permissions: ['site.manage'],
  email: 'editor@example.test',
};

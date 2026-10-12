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
  has_password: true,
  has_github: false,
  created_at: '2026-10-01T00:00:00Z',
  updated_at: '2026-10-01T00:00:00Z',
  last_login_at: null,
  access_status: 'ACTIVE',
  deletion_pending: false,
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

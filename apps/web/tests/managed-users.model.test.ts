import assert from 'node:assert/strict';
import test from 'node:test';

import {
  createUserDraft,
  draftChanges,
  finishAuthorization,
  prepareAuthorization,
} from '../src/application/management/user-editor.model.ts';
import {
  authorizationAccess,
  filterUsers,
  permissionSummary,
  usersReturnPath,
} from '../src/application/management/users.model.ts';

import { managedUser, systemActor } from './fixtures/managed-users.ts';

test('user filters combine case-insensitive identity, role and state with bounded pages', () => {
  const users = Array.from({ length: 43 }, (_, index) => ({
    ...managedUser,
    id: String(index),
    username: `member${index}`,
  }));
  const first = filterUsers(users, new URLSearchParams());
  assert.equal(first.users.length, 20);
  assert.equal(first.total, 43);
  assert.equal(first.pages, 3);
  assert.equal(filterUsers(users, new URLSearchParams('page=900')).users.length, 3);
  assert.equal(filterUsers(users, new URLSearchParams('page=NaN')).page, 1);
  assert.equal(filterUsers(users, new URLSearchParams('page=1.5')).page, 1);
  assert.equal(
    filterUsers(users, new URLSearchParams('q=EDITOR@EXAMPLE.TEST&role=ADMIN&state=active')).total,
    43,
  );
  assert.equal(filterUsers(users, new URLSearchParams('state=inactive')).total, 0);
  assert.equal(filterUsers(users, new URLSearchParams('role=USER')).total, 0);
  assert.equal(filterUsers(users, new URLSearchParams('q=member42')).users[0]?.id, '42');
  assert.equal(
    filterUsers([{ ...managedUser, email: null }], new URLSearchParams('q=编辑')).total,
    1,
  );
  assert.equal(filterUsers([], new URLSearchParams('page=2')).page, 1);
});

test('return navigation permits only list paths and known filter parameters', () => {
  for (const path of [
    'https://evil.test',
    '//evil.test',
    '/management/users/target',
    '/management/users-evil',
    '/management/users\\evil',
    'mailto:admin@example.test',
  ]) {
    assert.equal(usersReturnPath(path), '/management/users');
  }
  assert.equal(
    usersReturnPath('/management/users?q=编辑&role=ADMIN&page=2&error=raw#fragment'),
    '/management/users?q=%E7%BC%96%E8%BE%91&role=ADMIN&page=2',
  );
});

test('authorization matrix protects self, role scope and existing out-of-scope grants', () => {
  const manager = {
    ...managedUser,
    id: 'manager',
    permissions: ['user.manage', 'site.manage'] as const,
  };
  assert.equal(authorizationAccess(systemActor, managedUser).roleReason, null);
  assert.ok(authorizationAccess(systemActor, systemActor).permissionsReason);
  assert.ok(authorizationAccess(manager, managedUser).roleReason);
  assert.equal(authorizationAccess(manager, managedUser).permissionsReason, null);
  assert.match(
    authorizationAccess(manager, { ...managedUser, permissions: ['log.read'] }).permissionsReason ??
      '',
    /超出/,
  );
  assert.ok(authorizationAccess(manager, { ...managedUser, role: 'USER' }).permissionsReason);
  assert.ok(authorizationAccess(manager, systemActor).permissionsReason);
  assert.equal(permissionSummary(systemActor), '全部管理权限');
  assert.equal(
    authorizationAccess(systemActor, { ...systemActor, id: 'other-system' }).roleReason,
    null,
  );
});

test('draft submission refuses unchanged, overlapping, unauthorized and in-flight changes', () => {
  const draft = createUserDraft(managedUser);
  assert.equal(prepareAuthorization(systemActor, draft, 'permissions'), null);
  const permissions = { ...draft, permissions: [] };
  assert.deepEqual(prepareAuthorization(systemActor, permissions, 'permissions'), {
    kind: 'permissions',
    permissions: [],
  });
  assert.equal(
    prepareAuthorization(systemActor, { ...permissions, busy: true }, 'permissions'),
    null,
  );
  const mixed = { ...permissions, role: 'USER' as const };
  assert.equal(prepareAuthorization(systemActor, mixed, 'permissions'), null);
  assert.equal(prepareAuthorization(systemActor, mixed, 'role'), null);
  assert.equal(prepareAuthorization(managedUser, permissions, 'permissions'), null);
  const manager = { ...managedUser, id: 'manager', permissions: ['user.manage'] as const };
  assert.equal(
    prepareAuthorization(manager, { ...draft, permissions: ['log.read'] }, 'permissions'),
    null,
  );
});

test('success replaces the whole baseline; errors retain draft and block uncertain retries', () => {
  const draft = { ...createUserDraft(managedUser), role: 'USER' as const, busy: true };
  const saved = { ...managedUser, role: 'USER' as const, permissions: [] };
  const success = finishAuthorization(draft, { ok: true, user: saved });
  assert.deepEqual(success.permissions, []);
  assert.deepEqual(draftChanges(success), { role: false, permissions: false });
  assert.equal(success.busy, false);
  const failure = finishAuthorization(draft, { ok: false, code: 'network_error', status: 0 });
  assert.equal(failure.role, 'USER');
  assert.equal(failure.saved.role, 'ADMIN');
  assert.equal(failure.reloadRequired, true);
  assert.equal(prepareAuthorization(systemActor, failure, 'role'), null);
  assert.equal(
    finishAuthorization(draft, { ok: false, code: 'forbidden', status: 401 }).loginRequired,
    true,
  );
});

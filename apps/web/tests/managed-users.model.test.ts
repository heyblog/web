import assert from 'node:assert/strict';
import test from 'node:test';

import { managementNavigation } from '../src/application/management/navigation.ts';
import {
  createUserDraft,
  draftChanges,
  finishAuthorization,
  prepareAuthorization,
} from '../src/application/management/user-editor.model.ts';
import {
  accountManagementReason,
  authorizationAccess,
  permissionSummary,
  userMutationNeedsReload,
  usersQuery,
  usersReturnPath,
} from '../src/application/management/users.model.ts';

import { managedUser, systemActor } from './fixtures/managed-users.ts';

test('server list query bounds page and preserves accepted search filters', () => {
  assert.equal(
    usersQuery(new URLSearchParams('q= editor &role=ADMIN&status=ACTIVE&page=2')).toString(),
    'q=editor&role=ADMIN&status=ACTIVE&page=2&page_size=20',
  );
  assert.equal(
    usersQuery(new URLSearchParams('page=NaN&role=bad&status=bad')).toString(),
    'page=1&page_size=20',
  );
  assert.equal(usersQuery(new URLSearchParams('page=1.5')).get('page'), '1');
});

test('account changes follow role hierarchy and account lifecycle', () => {
  const manager = { ...managedUser, id: 'manager', permissions: ['user.manage'] as const };
  assert.equal(accountManagementReason(systemActor, managedUser), null);
  assert.equal(accountManagementReason(manager, { ...managedUser, role: 'USER' }), null);
  assert.ok(accountManagementReason(manager, managedUser));
  assert.ok(accountManagementReason(systemActor, systemActor));
  assert.ok(accountManagementReason(systemActor, { ...managedUser, deletion_pending: true }));
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
    updated_at: managedUser.updated_at,
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

test('management navigation uses role and business permission together', () => {
  assert.deepEqual(
    managementNavigation({ role: 'USER', permissions: ['user.manage', 'site_audit.review'] }),
    [],
  );
  assert.deepEqual(
    managementNavigation({ role: 'ADMIN', permissions: ['taxonomy.manage'] }).map(
      (item) => item.href,
    ),
    ['/management/tags'],
  );
  assert.ok(
    managementNavigation({ role: 'ADMIN', permissions: ['announcement.manage'] }).some(
      (item) => item.href === '/management/announcements',
    ),
  );
});

test('definite identity conflicts keep drafts editable while stale or uncertain outcomes require reload', () => {
  for (const code of ['username_taken', 'email_taken'])
    assert.equal(userMutationNeedsReload(code, 409), false);
  assert.equal(userMutationNeedsReload('validation_failed', 422), false);
  for (const [code, status] of [
    ['user_conflict', 409],
    ['network_error', 0],
    ['bad_gateway', 502],
    ['invalid_response', 200],
  ] as const)
    assert.equal(userMutationNeedsReload(code, status), true);
});

test('pending deletion locks account and authorization changes independently of access status', () => {
  const pending = { ...managedUser, deletion_pending: true, access_status: 'ACTIVE' };
  assert.ok(accountManagementReason(systemActor, pending));
  const access = authorizationAccess(systemActor, pending);
  assert.ok(access.roleReason);
  assert.ok(access.permissionsReason);
  assert.equal(
    prepareAuthorization(systemActor, { ...createUserDraft(pending), role: 'USER' }, 'role'),
    null,
  );
  assert.equal(
    prepareAuthorization(
      systemActor,
      { ...createUserDraft(pending), permissions: [] },
      'permissions',
    ),
    null,
  );
});

test('user queries stay within API search and page bounds', () => {
  const query = usersQuery(new URLSearchParams({ q: 'q'.repeat(200), page: '1000001' }));
  assert.equal(query.get('q')?.length, 128);
  assert.equal(query.get('page'), '1000000');
});

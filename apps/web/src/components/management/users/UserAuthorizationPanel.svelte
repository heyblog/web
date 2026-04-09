<script lang="ts">
  import type { ManagementPermissionKey, SessionUser } from '@/application/auth/auth.guard';
  import {
    type ManagedUserSnapshot,
    submitPermissionAuthorizationAction,
    submitRoleAuthorizationAction,
  } from '@/application/management/user-authorization.browser';
  import { openAlertDialog, openConfirmDialog } from '@/shared/browser/dialog.service';
  import { openToast } from '@/shared/browser/toast.service';

  import UserAuthorizationDialog from './UserAuthorizationDialog.svelte';
  import UserAuthorizationTable from './UserAuthorizationTable.svelte';

  let {
    users = [],
    currentUser = null,
  }: {
    users?: ManagedUserSnapshot[];
    currentUser?: SessionUser | null;
  } = $props();

  const permissionLabelByKey = new Map([
    ['user.manage', '用户与授权'],
    ['site_audit.review', '站点审核'],
    ['feedback.review', '反馈处理'],
    ['announcement.manage', '公告管理'],
    ['taxonomy.manage', '标签与目录'],
    ['site.manage', '站点编辑'],
    ['task.manage', '任务中心'],
    ['log.read', '日志查看'],
  ] satisfies Array<[ManagementPermissionKey, string]>);

  const cloneManagedUser = (entry: ManagedUserSnapshot): ManagedUserSnapshot => ({
    ...entry,
    permissions: [...entry.permissions],
  });

  let entries = $derived(users.map(cloneManagedUser));
  let selectedUserId = $state<string | null>(null);
  let modalOpen = $state(false);
  let modalBusy = $state(false);
  let modalError = $state('');
  let draftPermissions = $state<ManagementPermissionKey[]>([]);

  const actorIsSysAdmin = $derived(currentUser?.role === 'SYS_ADMIN');
  const actorPermissionSet = $derived(new Set(currentUser?.permissions ?? []));
  const selectedUser = $derived(entries.find((entry) => entry.id === selectedUserId) ?? null);

  const isCurrentUser = (entry: ManagedUserSnapshot): boolean => currentUser?.id === entry.id;

  const hasOutOfScopePermission = (entry: ManagedUserSnapshot): boolean =>
    !actorIsSysAdmin && entry.permissions.some((permission) => !actorPermissionSet.has(permission));

  const readPermissionSummary = (entry: ManagedUserSnapshot): string => {
    if (entry.role === 'SYS_ADMIN') {
      return '全部模块（隐式拥有）';
    }

    if (entry.role === 'USER') {
      return '普通用户不可持有模块权限';
    }

    if (entry.permissions.length === 0) {
      return '未授权模块';
    }

    return entry.permissions
      .map((permission) => permissionLabelByKey.get(permission) ?? permission)
      .join(' / ');
  };

  const readGrantSourceSummary = (entry: ManagedUserSnapshot): string => {
    const grantorId = entry.adminGrantedBy?.trim();

    if (!grantorId) {
      return 'system / self';
    }

    if (grantorId === entry.id) {
      return `${entry.nickname}（self）`;
    }

    const grantor = entries.find((item) => item.id === grantorId);

    if (grantor) {
      return `${grantor.nickname}`;
    }

    return '未知管理员';
  };

  const normalizePermissions = (
    permissions: ManagementPermissionKey[],
  ): ManagementPermissionKey[] => [...new Set(permissions)].sort();

  const updateEntry = (next: ManagedUserSnapshot): void => {
    entries = entries.map((entry) => (entry.id === next.id ? next : entry));
  };

  const openAuthorizationModal = (entry: ManagedUserSnapshot): void => {
    selectedUserId = entry.id;
    modalError = '';
    draftPermissions = [...entry.permissions].sort();
    modalOpen = true;
  };

  const closeAuthorizationModal = (): void => {
    modalBusy = false;
    modalError = '';
    selectedUserId = null;
    draftPermissions = [];
    modalOpen = false;
  };

  const updateDraftPermissions = (permissions: ManagementPermissionKey[]): void => {
    draftPermissions = normalizePermissions(permissions);
    modalError = '';
  };

  const jumpToLogin = (target: string | null): void => {
    window.location.assign(target?.trim() || '/login?next=%2Fmanagement%2Fusers');
  };

  const applyActionFailure = async (message: string): Promise<void> => {
    modalError = message;
    openToast({ tone: 'error', title: '授权操作失败', message });
    await openAlertDialog({
      title: '授权操作失败',
      description: message,
      tone: 'danger',
      confirmLabel: '我知道了',
    });
  };

  const ensureEditableAdminUser = async (entry: ManagedUserSnapshot | null): Promise<boolean> => {
    if (!entry || entry.role !== 'ADMIN') {
      await applyActionFailure('只有 ADMIN 用户可以编辑模块权限。');
      return false;
    }

    if (isCurrentUser(entry)) {
      await applyActionFailure('当前账号不可在此页面修改自身权限。');
      return false;
    }

    if (hasOutOfScopePermission(entry)) {
      await applyActionFailure('该用户包含超出你授权范围的权限，请由更高权限账号处理。');
      return false;
    }

    return true;
  };

  const handleGrantAdmin = async (): Promise<void> => {
    const entry = selectedUser;

    if (!entry) return;

    const confirmed = await openConfirmDialog({
      title: `确认提权 ${entry.nickname}?`,
      description: '提权后可继续为该用户分配模块权限。',
      tone: 'warning',
      confirmLabel: '确认提权',
      cancelLabel: '取消',
    });
    if (!confirmed) return;

    modalBusy = true;
    const result = await submitRoleAuthorizationAction(entry.id, 'grant-admin');
    modalBusy = false;

    if (result.redirect) return jumpToLogin(result.redirect);
    if (!result.ok || !result.data) return applyActionFailure(result.message);

    updateEntry(result.data);
    draftPermissions = [...result.data.permissions].sort();
    openToast({
      tone: 'success',
      title: '提权成功',
      message: `${result.data.nickname} 已提权为 ADMIN。`,
    });
  };

  const handleRevokeAdmin = async (): Promise<void> => {
    const entry = selectedUser;
    if (!entry) return;
    if (!(await ensureEditableAdminUser(entry))) return;

    const confirmed = await openConfirmDialog({
      title: `确认回收 ${entry.nickname} 的管理员身份?`,
      description: '回收后会清空该用户的全部模块权限。',
      tone: 'danger',
      confirmLabel: '确认回收',
      cancelLabel: '取消',
    });
    if (!confirmed) return;

    modalBusy = true;
    const result = await submitRoleAuthorizationAction(entry.id, 'revoke-admin');
    modalBusy = false;

    if (result.redirect) return jumpToLogin(result.redirect);
    if (!result.ok || !result.data) return applyActionFailure(result.message);

    updateEntry(result.data);
    openToast({
      tone: 'success',
      title: '回收成功',
      message: `${result.data.nickname} 已回收为 USER。`,
    });
    closeAuthorizationModal();
  };

  const handleSavePermissions = async (): Promise<void> => {
    const entry = selectedUser;
    if (!entry) return;
    if (!(await ensureEditableAdminUser(entry))) return;

    modalBusy = true;
    const result = await submitPermissionAuthorizationAction(entry.id, draftPermissions);
    modalBusy = false;

    if (result.redirect) return jumpToLogin(result.redirect);
    if (!result.ok || !result.data) return applyActionFailure(result.message);

    updateEntry(result.data);
    draftPermissions = [...result.data.permissions].sort();
    modalError = '';
    openToast({
      tone: 'success',
      title: '权限已保存',
      message: `${result.data.nickname} 的模块权限已更新。`,
    });
  };
</script>

<UserAuthorizationTable
  {entries}
  currentUserId={currentUser?.id ?? null}
  {selectedUserId}
  {modalOpen}
  {readPermissionSummary}
  {readGrantSourceSummary}
  onOpen={openAuthorizationModal}
/>

<UserAuthorizationDialog
  open={modalOpen}
  {selectedUser}
  {modalBusy}
  {modalError}
  {draftPermissions}
  showScopeHint={!actorIsSysAdmin}
  {hasOutOfScopePermission}
  onClose={closeAuthorizationModal}
  onGrantAdmin={handleGrantAdmin}
  onSavePermissions={handleSavePermissions}
  onRevokeAdmin={handleRevokeAdmin}
  onDraftPermissionsChange={updateDraftPermissions}
/>

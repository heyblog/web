<script lang="ts">
  import {
    MANAGEMENT_NAV_ITEMS,
    type ManagementPermissionKey,
  } from '@/application/auth/auth.guard';
  import type { ManagedUserSnapshot } from '@/application/management/user-authorization.browser';
  import FormMessage from '@/shared/ui/FormMessage.svelte';
  import ModalSurface from '@/shared/ui/ModalSurface.svelte';

  let {
    open = false,
    selectedUser = null,
    modalBusy = false,
    modalError = '',
    draftPermissions = [],
    showScopeHint = false,
    hasOutOfScopePermission,
    onClose,
    onGrantAdmin,
    onSavePermissions,
    onRevokeAdmin,
    onDraftPermissionsChange,
  }: {
    open?: boolean;
    selectedUser?: ManagedUserSnapshot | null;
    modalBusy?: boolean;
    modalError?: string;
    draftPermissions?: ManagementPermissionKey[];
    showScopeHint?: boolean;
    hasOutOfScopePermission?: (entry: ManagedUserSnapshot) => boolean;
    onClose?: () => void;
    onGrantAdmin?: () => void;
    onSavePermissions?: () => void;
    onRevokeAdmin?: () => void;
    onDraftPermissionsChange?: (permissions: ManagementPermissionKey[]) => void;
  } = $props();

  const currentHasOutOfScope = (entry: ManagedUserSnapshot): boolean =>
    hasOutOfScopePermission?.(entry) ?? false;

  const normalizePermissions = (
    permissions: ManagementPermissionKey[],
  ): ManagementPermissionKey[] => [...new Set(permissions)].sort();

  const handlePermissionChange = (permission: ManagementPermissionKey, event: Event): void => {
    const input = event.currentTarget as HTMLInputElement;

    const nextPermissions = normalizePermissions(
      input.checked
        ? [...draftPermissions, permission]
        : draftPermissions.filter((item) => item !== permission),
    );
    onDraftPermissionsChange?.(nextPermissions);
  };
</script>

<ModalSurface
  {open}
  title={selectedUser ? `授权设置 · ${selectedUser.nickname}` : '授权设置'}
  description="在弹窗中完成提权、回收和模块权限调整，提交后即时生效。"
  showCancel={false}
  showConfirm={false}
  showFooter={false}
  showHeaderClose={true}
  headerCloseAriaLabel="关闭授权弹窗"
  onCancel={onClose}
>
  {#if selectedUser}
    <div class="space-y-4">
      {#if modalError}
        <FormMessage tone="error" title="需要处理" message={modalError} />
      {/if}

      {#if selectedUser.role === 'USER'}
        <FormMessage
          tone="info"
          title="当前是普通用户"
          message="请先执行提权，再为该用户分配模块权限。"
        />
        <button
          class="inline-flex items-center rounded-sm border border-(--color-line-med) px-3 py-1.5 text-xs font-medium"
          type="button"
          disabled={modalBusy}
          onclick={onGrantAdmin}
        >
          {modalBusy ? '处理中…' : '提权为 ADMIN'}
        </button>
      {:else}
        {#if currentHasOutOfScope(selectedUser)}
          <FormMessage
            tone="warning"
            title="权限范围受限"
            message="该用户当前包含超出你授权范围的权限，请由更高权限账号处理。"
          />
        {/if}

        <div class="permission-list">
          {#each MANAGEMENT_NAV_ITEMS as item (item.permission)}
            {@const selected = draftPermissions.includes(item.permission)}
            <label class="permission-option" class:permission-option-selected={selected}>
              <input
                class="permission-input"
                type="checkbox"
                checked={selected}
                disabled={modalBusy}
                onchange={(event) => handlePermissionChange(item.permission, event)}
              />
              <strong class="permission-option-label">{item.label}</strong>
              <small class="permission-option-key" title={item.permission}>{item.permission}</small>
            </label>
          {/each}
        </div>

        {#if showScopeHint}
          <p class="text-xs text-(--color-fg-3)">仅可分配你当前已持有的模块权限。</p>
        {/if}

        <div class="mt-2 flex flex-wrap justify-end gap-2">
          <button
            class="action-button action-button-danger min-h-0 px-3 py-1.5 text-xs"
            type="button"
            disabled={modalBusy}
            onclick={onRevokeAdmin}
          >
            回收为 USER
          </button>
          <button
            class="action-button action-button-secondary min-h-0 px-3 py-1.5 text-xs"
            type="button"
            disabled={modalBusy}
            onclick={onSavePermissions}
          >
            {modalBusy ? '处理中…' : '保存模块权限'}
          </button>
        </div>
      {/if}
    </div>
  {/if}
</ModalSurface>

<style>
  .permission-list {
    display: grid;
    gap: 8px;
  }

  .permission-option {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: nowrap;
    border: 1px solid var(--color-line-med);
    border-radius: var(--radius-sm);
    padding: 10px 12px;
    background: color-mix(in srgb, var(--color-bg) 84%, var(--color-bg-raised));
    cursor: pointer;
    transition:
      border-color 160ms ease,
      background-color 160ms ease,
      box-shadow 160ms ease;
  }

  .permission-option:hover {
    border-color: var(--color-line-strong);
  }

  .permission-option:focus-within {
    border-color: color-mix(in srgb, var(--color-info) 60%, var(--color-line-med));
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-info) 12%, transparent);
  }

  .permission-option-selected {
    border-color: color-mix(in srgb, var(--color-info) 58%, var(--color-line-med));
    background: color-mix(in srgb, var(--color-info) 11%, var(--color-bg-raised));
  }

  .permission-option-label {
    font-size: 14px;
    font-weight: 600;
    color: var(--color-fg);
    white-space: nowrap;
  }

  .permission-option-key {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 12px;
    line-height: 1.5;
    color: var(--color-fg-3);
  }

  .permission-option .permission-input {
    position: static;
    width: 16px;
    height: 16px;
    margin: 0;
    border: 0;
    padding: 0;
    overflow: visible;
    clip-path: none;
    white-space: normal;
    appearance: auto;
    accent-color: var(--color-info);
  }
</style>

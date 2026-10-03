<script lang="ts">
  import { onMount, untrack } from 'svelte';

  import { type ManagementPermission, managementPermissions } from '@/api/auth/auth.types';
  import { saveAuthorization } from '@/api/managed-users/managed-users.browser';
  import type {
    AuthorizationOperation,
    ManagedUser,
  } from '@/api/managed-users/managed-users.types';
  import {
    createUserDraft,
    draftChanges,
    finishAuthorization,
    prepareAuthorization,
  } from '@/application/management/user-editor.model';
  import { authorizationAccess, permissionLabels } from '@/application/management/users.model';
  import { protectUserDraft } from '@/application/management/users-navigation.browser';

  import { buttonClass, inputClass, panelClass, primaryClass } from './users.styles';
  import UserSummary from './UserSummary.svelte';

  interface Props {
    actor: ManagedUser;
    user: ManagedUser;
    returnPath: string;
  }
  let { actor, user, returnPath }: Props = $props();
  let draft = $state(untrack(() => createUserDraft(user)));
  let confirmed = $state(false);
  const dirty = $derived(draftChanges(draft));
  const access = $derived(authorizationAccess(actor, draft.saved));
  const locked = $derived(draft.busy || draft.reloadRequired);
  const lowering = $derived(
    dirty.role && (draft.role === 'USER' || draft.saved.role === 'SYS_ADMIN'),
  );
  const roleDisabled = $derived(Boolean(access.roleReason) || locked || dirty.permissions);
  const permissionsDisabled = $derived(Boolean(access.permissionsReason) || locked || dirty.role);

  onMount(() =>
    protectUserDraft(() => ({ dirty: dirty.role || dirty.permissions, busy: draft.busy })),
  );

  function toggle(permission: ManagementPermission, checked: boolean) {
    draft = {
      ...draft,
      permissions: checked
        ? [...draft.permissions, permission]
        : draft.permissions.filter((value) => value !== permission),
      error: null,
      notice: null,
    };
  }

  async function submit(kind: AuthorizationOperation) {
    if (kind === 'role' && lowering && !confirmed) return;
    const change = prepareAuthorization(actor, draft, kind);
    if (!change) return;
    draft = { ...draft, busy: true, error: null, notice: null };
    const result = await saveAuthorization(draft.saved.id, change);
    draft = finishAuthorization(draft, result);
    if (result.ok) confirmed = false;
  }

  function reset() {
    draft = createUserDraft(draft.saved);
    confirmed = false;
  }
</script>

<div class="mb-6 flex flex-wrap items-center justify-between gap-3">
  <a
    class="inline-flex min-h-11 items-center text-sm font-medium text-tint-fg hover:underline sm:min-h-10"
    href={returnPath}>← 返回用户列表</a
  >
  {#if dirty.role || dirty.permissions}<span class="text-xs text-warning-fg">有未保存的修改</span
    >{/if}
</div>
<UserSummary user={draft.saved} {actor} />
{#if draft.error}<div
    class="mb-6 rounded-md border border-danger bg-danger-bg p-4 text-sm text-danger-fg"
    role="alert"
  >
    <p>{draft.error}</p>
    {#if draft.reloadRequired}<a
        class="mt-2 inline-flex min-h-11 items-center font-medium underline sm:min-h-10"
        href={draft.loginRequired
          ? `/login?next=${encodeURIComponent(`/management/users/${draft.saved.id}`)}`
          : `/management/users/${draft.saved.id}?returnTo=${encodeURIComponent(returnPath)}`}
        >{draft.loginRequired ? '重新登录' : '重新加载'}</a
      >{/if}
  </div>{/if}
{#if draft.notice}<p
    class="mb-6 rounded-md border border-success bg-success-bg p-4 text-sm text-success-fg"
    role="status"
  >
    {draft.notice}
  </p>{/if}
<noscript><p class="mb-6 text-sm text-fg-muted">编辑授权需要启用 JavaScript。</p></noscript>
<div class="grid items-start gap-6 md:grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)]">
  <section class={panelClass} aria-labelledby="role-heading">
    <h2 id="role-heading" class="text-lg font-semibold">角色设置</h2>
    <p class="mt-2 mb-5 text-sm text-fg-muted">管理员可获得模块权限。</p>
    <form
      method="post"
      action={`/management/users/${draft.saved.id}/role`}
      onsubmit={(event) => {
        event.preventDefault();
        void submit('role');
      }}
      aria-busy={draft.busy}
    >
      <label class="grid gap-2 text-sm font-medium" for="user-role"
        >角色
        <select
          id="user-role"
          name="role"
          class={inputClass}
          disabled={roleDisabled}
          value={draft.role}
          onchange={(event) => {
            const value = event.currentTarget.value;
            if (value === 'USER' || value === 'ADMIN' || value === 'SYS_ADMIN')
              draft = { ...draft, role: value, error: null, notice: null };
            confirmed = false;
          }}
        >
          {#if draft.saved.role === 'SYS_ADMIN'}<option value="SYS_ADMIN" disabled
              >系统管理员</option
            >{/if}
          <option value="USER">普通用户</option><option value="ADMIN">管理员</option>
        </select>
      </label>
      {#if access.roleReason}<p class="mt-3 text-sm text-fg-muted">{access.roleReason}</p>
      {:else if dirty.permissions}<p class="mt-3 text-sm text-fg-muted">
          请先保存或撤销模块权限修改。
        </p>{/if}
      {#if lowering}<div
          class="mt-4 rounded-md border border-warning-border bg-warning-bg p-4 text-sm text-warning-fg"
        >
          <p>
            {draft.role === 'USER'
              ? draft.saved.role === 'SYS_ADMIN'
                ? '改为普通用户后，将失去系统管理员身份并清空模块权限。该用户需要重新登录，此页面无法恢复系统管理员角色。'
                : '改为普通用户后，将清空模块权限，该用户需要重新登录。'
              : '改为管理员后，将失去系统管理员权限，该用户需要重新登录。此页面无法恢复系统管理员角色。'}
          </p>
          <label class="mt-2 flex min-h-11 items-center gap-2 sm:min-h-10"
            ><input
              class="size-4 shrink-0 accent-primary"
              type="checkbox"
              bind:checked={confirmed}
              disabled={locked}
            />确认降低角色</label
          >
        </div>{/if}
      {#if !access.roleReason}<div class="mt-6 flex flex-wrap gap-3 border-t border-line pt-4">
          <button
            class={dirty.role ? primaryClass : buttonClass}
            type="submit"
            disabled={roleDisabled || !dirty.role || (lowering && !confirmed)}
            >{draft.busy && dirty.role ? '保存中…' : '保存角色'}</button
          >
          {#if dirty.role}<button
              class={buttonClass}
              type="button"
              disabled={locked}
              onclick={reset}>撤销修改</button
            >{/if}
        </div>{/if}
    </form>
  </section>
  <section class={panelClass} aria-labelledby="permissions-heading">
    <div class="mb-5 flex flex-wrap items-center justify-between gap-2">
      <h2 id="permissions-heading" class="text-lg font-semibold">模块权限</h2>
      <span class="text-xs text-fg-muted"
        >{draft.saved.role === 'SYS_ADMIN'
          ? '全部管理权限'
          : `已选 ${draft.permissions.length} 项`}</span
      >
    </div>
    {#if access.permissionsReason}<p class="mb-4 text-sm text-fg-muted">
        {access.permissionsReason}
      </p>
    {:else if dirty.role}<p class="mb-4 text-sm text-fg-muted">请先保存或撤销角色修改。</p>{/if}
    <form
      method="post"
      action={`/management/users/${draft.saved.id}/permissions`}
      onsubmit={(event) => {
        event.preventDefault();
        void submit('permissions');
      }}
      aria-busy={draft.busy}
    >
      <fieldset class="grid gap-x-6 gap-y-1 sm:grid-cols-2" disabled={permissionsDisabled}>
        <legend class="sr-only">{draft.saved.display_name} 的模块权限</legend>
        {#each managementPermissions as permission (permission)}
          {@const assignable = actor.role === 'SYS_ADMIN' || actor.permissions.includes(permission)}
          <label class="flex min-h-11 items-center gap-3 text-sm sm:min-h-10">
            <input
              class="size-4 shrink-0 accent-primary"
              type="checkbox"
              name="permissions"
              value={permission}
              checked={draft.saved.role === 'SYS_ADMIN' || draft.permissions.includes(permission)}
              disabled={!assignable}
              onchange={(event) => toggle(permission, event.currentTarget.checked)}
            />
            <span
              >{permissionLabels[permission]}{#if !assignable && !access.permissionsReason}<span
                  class="ml-1 text-xs text-fg-muted">不可授予</span
                >{/if}</span
            >
          </label>
        {/each}
      </fieldset>
      {#if !access.permissionsReason}<div class="mt-6 border-t border-line pt-4">
          <p class="mb-4 text-xs text-fg-muted">保存权限后，该用户需要重新登录。</p>
          <div class="flex flex-wrap gap-3">
            <button
              class={dirty.permissions ? primaryClass : buttonClass}
              type="submit"
              disabled={permissionsDisabled || !dirty.permissions}
              >{draft.busy && dirty.permissions ? '保存中…' : '保存权限'}</button
            >
            {#if dirty.permissions}<button
                class={buttonClass}
                type="button"
                disabled={locked}
                onclick={reset}>撤销修改</button
              >{/if}
          </div>
        </div>{/if}
    </form>
  </section>
</div>

<script lang="ts">
  import type { ManagedUserSnapshot } from '@/application/management/user-authorization.browser';

  let {
    entries = [],
    currentUserId = null,
    selectedUserId = null,
    modalOpen = false,
    onOpen,
    readPermissionSummary,
    readGrantSourceSummary,
  }: {
    entries?: ManagedUserSnapshot[];
    currentUserId?: string | null;
    selectedUserId?: string | null;
    modalOpen?: boolean;
    onOpen?: (entry: ManagedUserSnapshot) => void;
    readPermissionSummary?: (entry: ManagedUserSnapshot) => string;
    readGrantSourceSummary?: (entry: ManagedUserSnapshot) => string;
  } = $props();

  const isCurrentUser = (entry: ManagedUserSnapshot): boolean => currentUserId === entry.id;
</script>

<div class="overflow-x-auto rounded-sm border border-(--color-line)">
  <table class="min-w-full border-collapse text-left text-sm">
    <thead class="border-b border-(--color-line) bg-(--color-bg-raised)">
      <tr>
        <th class="px-4 py-3 align-middle font-medium">用户</th>
        <th class="px-4 py-3 align-middle font-medium">角色</th>
        <th class="px-4 py-3 align-middle font-medium">模块权限</th>
        <th class="px-4 py-3 align-middle font-medium">最近登录</th>
        <th class="px-4 py-3 align-middle font-medium">操作</th>
      </tr>
    </thead>
    <tbody>
      {#each entries as entry (entry.id)}
        <tr
          class:list={[
            'border-b border-(--color-line) last:border-b-0',
            selectedUserId === entry.id && modalOpen && 'bg-(--color-bg-raised)',
          ]}
        >
          <td class="px-4 py-4 align-middle">
            <div class="font-medium">{entry.nickname}</div>
            <div class="mt-1 text-xs text-(--color-fg-3)">{entry.email}</div>
            <div class="mt-1 text-xs text-(--color-fg-3)">
              授权来源：{readGrantSourceSummary?.(entry) ?? 'system / self'}
            </div>
          </td>
          <td class="px-4 py-4 align-middle">
            <div>{entry.role}</div>
            <div class="mt-1 text-xs text-(--color-fg-3)">
              授权时间：{entry.adminGrantedTime ?? '—'}
            </div>
          </td>
          <td class="px-4 py-4 align-middle text-xs text-(--color-fg-3)">
            {readPermissionSummary?.(entry)}
          </td>
          <td class="px-4 py-4 align-middle text-xs text-(--color-fg-3)">
            {entry.lastLoginTime ?? 'never'}
          </td>
          <td class="px-4 py-4 align-middle">
            {#if entry.role === 'SYS_ADMIN'}
              <span class="text-xs text-(--color-fg-3)">SYS_ADMIN 不可在此流程修改</span>
            {:else if isCurrentUser(entry)}
              <span class="text-xs text-(--color-fg-3)">当前账号不可自助改角色/权限</span>
            {:else}
              <button
                class="inline-flex items-center rounded-sm border border-(--color-line-med) px-3 py-1.5 text-xs font-medium"
                type="button"
                onclick={() => onOpen?.(entry)}
              >
                打开授权弹窗
              </button>
            {/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

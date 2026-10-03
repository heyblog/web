<script lang="ts">
  import type { ManagedUser } from '@/api/managed-users/managed-users.types';
  import { roleLabels } from '@/application/management/users.model';

  import { panelClass } from './users.styles';
  let { user, actor }: { user: ManagedUser; actor: ManagedUser } = $props();
</script>

<section class={`${panelClass} mb-6`} aria-label="用户信息">
  <div class="flex flex-wrap items-start justify-between gap-4">
    <div class="min-w-0">
      <h2 class="text-xl font-semibold wrap-anywhere">
        {user.display_name || user.username}
      </h2>
      <p class="mt-2 text-sm wrap-anywhere text-fg-muted">
        @{user.username} · {user.email || '未绑定邮箱'}
      </p>
    </div>
    <span class="rounded-sm bg-tint px-2 text-xs/5 font-medium text-tint-fg"
      >{roleLabels[user.role]}</span
    >
  </div>
  <div class="mt-4 flex flex-wrap gap-3 text-xs text-fg-muted">
    <span class={user.active ? 'text-success-fg' : ''}>{user.active ? '账号可用' : '账号停用'}</span
    >
    <span>{user.email_verified ? '邮箱已验证' : '邮箱未验证'}</span>
    {#if actor.id === user.id}<span>当前账号</span>{/if}
  </div>
</section>

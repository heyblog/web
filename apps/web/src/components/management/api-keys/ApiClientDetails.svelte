<script lang="ts">
  import type { ClientPanel } from '@/application/api-keys/api-keys.controller.svelte';
  import {
    activeKeyCount,
    formatCredentialDate,
    keyStatus,
  } from '@/application/api-keys/api-keys.model';
  import {
    apiClientScopeLabels,
    type ApiClientSummary,
  } from '@/application/api-keys/api-keys.types';

  import { buttonClass, dangerClass } from './api-keys.styles';
  interface Props {
    client: ApiClientSummary;
    now: number;
    busy: boolean;
    onaction: (panel: ClientPanel) => void;
  }
  let { client, now, busy, onaction }: Props = $props();
  const labels = {
    active: '有效',
    expired: '已过期',
    revoked: '已撤销',
    disabled: '调用方已停用',
  } as const;
  let activeCount = $derived(activeKeyCount(client, now));
</script>

<div class="grid gap-8 p-6">
  <section class="grid gap-4" aria-label="基本设置">
    <div class="flex items-center justify-between gap-3">
      <span class="text-sm text-fg-muted"
        >{client.audience === 'INTERNAL' ? '内部服务' : '外部服务'}</span
      ><span
        class={[
          'rounded-sm px-2 py-1 text-xs',
          client.disabled_at ? 'bg-subtle text-fg-muted' : 'bg-success-bg text-success-fg',
        ]}>{client.disabled_at ? '已停用' : '已启用'}</span
      >
    </div>
    <h3 class="text-lg font-semibold wrap-break-word">{client.name}</h3>
    {#if client.description}<p class="text-sm wrap-break-word text-fg-muted">
        {client.description}
      </p>{/if}
    <dl class="grid gap-3 text-sm">
      <div class="grid gap-2">
        <dt class="text-fg-muted">调用权限</dt>
        <dd class="flex flex-wrap gap-2">
          {#each client.scopes as scope (scope)}<span
              class="rounded-sm bg-tint px-2 py-1 text-xs text-tint-fg"
              >{apiClientScopeLabels[scope]}</span
            >{/each}
        </dd>
      </div>
      <div class="flex flex-wrap justify-between gap-2">
        <dt class="text-fg-muted">创建时间</dt>
        <dd>{formatCredentialDate(client.created_at)}</dd>
      </div>
    </dl>
    <div class="flex flex-wrap gap-3">
      <button
        class={buttonClass}
        disabled={busy}
        onclick={() => onaction({ kind: 'edit', clientId: client.id })}>编辑设置</button
      ><button
        class={client.disabled_at ? buttonClass : dangerClass}
        disabled={busy}
        onclick={() => onaction({ kind: 'toggle', clientId: client.id })}
        >{client.disabled_at ? '启用调用方' : '停用调用方'}</button
      >
    </div>
  </section>
  <section class="grid gap-4 border-t border-line pt-6" aria-labelledby="client-keys-heading">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <h3 id="client-keys-heading" class="font-semibold">
        密钥记录 <span class="ml-1 text-sm font-normal text-fg-muted">{client.keys.length}</span>
      </h3>
      <button
        class={buttonClass}
        disabled={busy || client.disabled_at !== null}
        onclick={() =>
          onaction({ kind: activeCount > 0 ? 'rotate' : 'issue', clientId: client.id })}
        >{activeCount > 0 ? '轮换密钥' : '签发新密钥'}</button
      >
    </header>
    {#if client.disabled_at}<p class="text-sm text-fg-muted">
        启用调用方后才能轮换或签发密钥。
      </p>{:else if activeCount === 0}<p class="text-sm text-fg-muted">
        当前没有有效密钥，可重新签发后继续调用。
      </p>{/if}
    {#each client.keys as key (key.id)}
      {@const status = keyStatus(key, client.disabled_at !== null, now)}
      <article class="grid gap-3 rounded-md border border-line p-4">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <code class="text-xs break-all">{key.prefix}…</code><span
            class={[
              'rounded-sm px-2 py-1 text-xs',
              status === 'active'
                ? 'bg-success-bg text-success-fg'
                : status === 'expired'
                  ? 'bg-warning-bg text-warning-fg'
                  : 'bg-subtle text-fg-muted',
            ]}>{labels[status]}</span
          >
        </div>
        <dl class="grid gap-2 text-xs">
          <div class="flex flex-wrap justify-between gap-2">
            <dt class="text-fg-muted">到期时间</dt>
            <dd>{formatCredentialDate(key.expires_at, '永不过期')}</dd>
          </div>
          <div class="flex flex-wrap justify-between gap-2">
            <dt class="text-fg-muted">最近使用</dt>
            <dd>{formatCredentialDate(key.last_used_at)}</dd>
          </div>
          <div class="flex flex-wrap justify-between gap-2">
            <dt class="text-fg-muted">创建时间</dt>
            <dd>{formatCredentialDate(key.created_at)}</dd>
          </div>
        </dl>
        {#if status === 'active' || status === 'disabled'}<button
            class={`${dangerClass} w-fit`}
            disabled={busy}
            onclick={() => onaction({ kind: 'revoke', clientId: client.id, key })}>撤销密钥</button
          >{/if}
      </article>
    {/each}
  </section>
</div>

<script lang="ts">
  import { IconArrowRight, IconKey } from '@tabler/icons-svelte';

  import {
    activeKeyCount,
    formatCredentialDate,
    lastClientUse,
  } from '@/application/api-keys/api-keys.model';
  import {
    apiClientScopeLabels,
    type ApiClientSummary,
  } from '@/application/api-keys/api-keys.types';

  import { buttonClass } from './api-keys.styles';
  interface Props {
    clients: readonly ApiClientSummary[];
    now: number;
    filtered: boolean;
    onselect: (client: ApiClientSummary) => void;
  }
  let { clients, now, filtered, onselect }: Props = $props();
</script>

{#snippet identity(client: ApiClientSummary)}
  <button
    class="min-h-11 text-left font-medium text-tint-fg underline-offset-4 hover:underline sm:min-h-10"
    onclick={() => onselect(client)}>{client.name}</button
  >
  {#if client.description}<p class="max-w-80 text-sm wrap-break-word text-fg-muted">
      {client.description}
    </p>{/if}
{/snippet}
{#snippet status(client: ApiClientSummary)}
  <span
    class={[
      'inline-flex rounded-sm px-2 py-1 text-xs font-medium',
      client.disabled_at ? 'bg-subtle text-fg-muted' : 'bg-success-bg text-success-fg',
    ]}>{client.disabled_at ? '已停用' : '已启用'}</span
  >
{/snippet}
{#snippet scopes(client: ApiClientSummary)}
  <div class="flex flex-wrap gap-2">
    {#each client.scopes as scope (scope)}<span
        class="rounded-sm bg-tint px-2 py-1 text-xs text-tint-fg"
        >{apiClientScopeLabels[scope]}</span
      >{/each}
  </div>
{/snippet}

{#if clients.length === 0}
  <div
    class="grid justify-items-center gap-3 rounded-md border border-line bg-surface px-6 py-12 text-center"
  >
    <div class="grid size-12 place-items-center rounded-md bg-subtle text-fg-muted">
      <IconKey size={24} stroke={1.8} aria-hidden="true" />
    </div>
    <p class="font-semibold">{filtered ? '没有符合条件的调用方' : '尚未创建调用方'}</p>
    <p class="text-sm text-fg-muted">
      {filtered ? '调整搜索词或筛选条件后重试。' : '创建调用方后，可为服务签发调用密钥。'}
    </p>
  </div>
{:else}
  <div class="hidden overflow-x-auto rounded-md border border-line bg-surface md:block">
    <table class="w-full text-left text-sm">
      <thead class="border-b border-line bg-subtle text-xs text-fg-muted"
        ><tr
          >{#each ['调用方', '类型', '调用权限', '状态', '有效密钥', '最近使用'] as heading (heading)}<th
              class="px-4 py-3 font-medium whitespace-nowrap">{heading}</th
            >{/each}</tr
        ></thead
      >
      <tbody class="divide-y divide-line"
        >{#each clients as client (client.id)}
          <tr class="transition-colors duration-(--motion-color) hover:bg-subtle/50">
            <td class="max-w-80 px-4 py-3 wrap-break-word">{@render identity(client)}</td>
            <td class="px-4 py-3 whitespace-nowrap"
              >{client.audience === 'INTERNAL' ? '内部服务' : '外部服务'}</td
            >
            <td class="px-4 py-3">{@render scopes(client)}</td>
            <td class="px-4 py-3 whitespace-nowrap">{@render status(client)}</td>
            <td class="px-4 py-3 font-mono tabular-nums">{activeKeyCount(client, now)}</td>
            <td class="px-4 py-3 text-xs whitespace-nowrap text-fg-muted"
              >{formatCredentialDate(lastClientUse(client))}</td
            >
          </tr>
        {/each}</tbody
      >
    </table>
  </div>
  <div class="divide-y divide-line rounded-md border border-line bg-surface md:hidden">
    {#each clients as client (client.id)}
      <article class="grid min-w-0 gap-3 p-5">
        <header class="flex items-start justify-between gap-3">
          <div class="min-w-0 wrap-break-word">{@render identity(client)}</div>
          {@render status(client)}
        </header>
        <div class="flex flex-wrap gap-x-4 gap-y-2 text-sm text-fg-muted">
          <span>{client.audience === 'INTERNAL' ? '内部服务' : '外部服务'}</span><span
            >有效密钥 {activeKeyCount(client, now)}</span
          >
        </div>
        {@render scopes(client)}
        <div class="flex items-center justify-between gap-3">
          <p class="text-xs text-fg-muted">
            最近使用：{formatCredentialDate(lastClientUse(client))}
          </p>
          <button
            class={buttonClass}
            onclick={() => onselect(client)}
            aria-label={`管理 ${client.name}`}
            ><IconArrowRight size={18} aria-hidden="true" /></button
          >
        </div>
      </article>
    {/each}
  </div>
{/if}

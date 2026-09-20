<script lang="ts">
  import { IconPlus, IconRefresh } from '@tabler/icons-svelte';
  import { onMount, untrack } from 'svelte';

  import { ApiKeyController } from '@/application/api-keys/api-keys.controller.svelte';
  import { type ClientFilters, filterClients } from '@/application/api-keys/api-keys.model';
  import type { ApiClientSummary } from '@/application/api-keys/api-keys.types';

  import {
    buttonClass,
    dangerClass,
    errorClass,
    inputClass,
    primaryClass,
  } from './api-keys.styles';
  import ApiClientDetails from './ApiClientDetails.svelte';
  import ApiClientForm from './ApiClientForm.svelte';
  import ApiClientList from './ApiClientList.svelte';
  import ApiCredentialResult from './ApiCredentialResult.svelte';
  import ApiKeyOperation from './ApiKeyOperation.svelte';
  import ApiKeyPanel from './ApiKeyPanel.svelte';

  interface Props {
    initialClients: readonly ApiClientSummary[];
    initialError?: string | null;
  }
  let { initialClients, initialError = null }: Props = $props();
  const controller = untrack(() => new ApiKeyController(initialClients));
  controller.refreshError = untrack(() => initialError);
  let query = $state('');
  let audience = $state<ClientFilters['audience']>('ALL');
  let status = $state<ClientFilters['status']>('ALL');
  let now = $state(Date.now());
  let clients = $derived(filterClients(controller.clients, { query, audience, status }));
  let client = $derived(controller.selected);
  let panel = $derived(controller.panel);
  let locked = $derived(controller.busy || controller.refreshing);
  let title = $derived(
    panel.kind === 'create'
      ? '创建调用方'
      : panel.kind === 'edit'
        ? '编辑调用方'
        : panel.kind === 'rotate'
          ? '轮换密钥'
          : panel.kind === 'issue'
            ? '签发新密钥'
            : panel.kind === 'toggle'
              ? client?.disabled_at
                ? '启用调用方'
                : '停用调用方'
              : panel.kind === 'revoke'
                ? '撤销密钥'
                : panel.kind === 'credential'
                  ? '密钥已生成'
                  : '调用方详情',
  );

  onMount(() => {
    const timer = window.setInterval(() => {
      now = Date.now();
    }, 30_000);
    return () => window.clearInterval(timer);
  });
</script>

<svelte:window
  onfocus={() => {
    now = Date.now();
  }}
/>

<section aria-label="调用凭证管理">
  <div class="mb-6 flex flex-wrap items-center justify-between gap-4">
    <p class="text-sm text-fg-muted">管理服务的调用权限与密钥。</p>
    <div class="flex gap-3">
      <button
        class={buttonClass}
        disabled={locked}
        aria-busy={controller.refreshing}
        onclick={() => controller.refresh()}
        ><IconRefresh size={17} aria-hidden="true" />刷新</button
      ><button
        class={primaryClass}
        disabled={locked}
        onclick={() => controller.open({ kind: 'create' })}
        ><IconPlus size={17} aria-hidden="true" />创建调用方</button
      >
    </div>
  </div>
  <div class="mb-5 grid gap-4 sm:grid-cols-[minmax(0,1fr)_10rem_10rem]">
    <label class="grid gap-2 text-sm font-medium"
      >搜索调用方<input
        class={inputClass}
        type="search"
        placeholder="名称或用途"
        bind:value={query}
      /></label
    >
    <label class="grid gap-2 text-sm font-medium"
      >调用类型<select class={inputClass} bind:value={audience}
        ><option value="ALL">全部类型</option><option value="INTERNAL">内部服务</option><option
          value="EXTERNAL">外部服务</option
        ></select
      ></label
    >
    <label class="grid gap-2 text-sm font-medium"
      >调用方状态<select class={inputClass} bind:value={status}
        ><option value="ALL">全部状态</option><option value="enabled">已启用</option><option
          value="disabled">已停用</option
        ></select
      ></label
    >
  </div>
  {#if controller.refreshError}<p class={`${errorClass} mb-4`} role="alert">
      列表未能刷新。{controller.refreshError}
    </p>{/if}
  {#if controller.notice}<p class="mb-4 text-sm text-success-fg" role="status">
      {controller.notice}
    </p>{/if}
  <div class="mb-3 flex items-center justify-between gap-3 text-xs text-fg-muted">
    <span role="status">{clients.length} 个调用方</span
    >{#if query || audience !== 'ALL' || status !== 'ALL'}<button
        class="min-h-11 text-tint-fg underline-offset-4 hover:underline sm:min-h-10"
        onclick={() => {
          query = '';
          audience = 'ALL';
          status = 'ALL';
        }}>清除筛选</button
      >{/if}
  </div>
  <ApiClientList
    {clients}
    {now}
    filtered={Boolean(query || audience !== 'ALL' || status !== 'ALL')}
    onselect={(selected) => controller.open({ kind: 'details', clientId: selected.id })}
  />
</section>

{#if panel.kind !== 'closed'}
  <ApiKeyPanel
    title={controller.discard ? '放弃尚未保存的修改？' : title}
    drawer={panel.kind !== 'create' && panel.kind !== 'credential'}
    busy={controller.busy}
    onrequestclose={() => controller.requestClose()}
  >
    {#if controller.discard}
      <div class="grid gap-5 p-6">
        <p class="text-sm text-fg-muted">未保存的内容将会丢失。</p>
        <div class="flex justify-end gap-3">
          <button
            class={buttonClass}
            onclick={() => {
              controller.discard = false;
            }}>继续编辑</button
          ><button class={dangerClass} onclick={() => controller.close()}>放弃修改</button>
        </div>
      </div>
    {/if}
    <div class={['flex min-h-0 flex-1 flex-col', controller.discard && 'hidden']}>
      {#if controller.error}<p class={`${errorClass} mx-6 mt-5`} role="alert">
          {controller.error}
        </p>{/if}
      {#if controller.refreshError}<div
          class="mx-6 mt-5 grid gap-2 rounded-md border border-warning-border bg-warning-bg p-3 text-sm text-warning-fg"
          role="alert"
        >
          <p>操作后的列表未能刷新。{controller.refreshError}</p>
          <button
            class={`${buttonClass} w-fit`}
            disabled={locked}
            onclick={() => controller.refresh()}>重新加载列表</button
          >
        </div>{/if}
      {#if panel.kind === 'create'}
        <ApiClientForm
          busy={locked}
          ondirty={(dirty) => {
            controller.dirty = dirty;
          }}
          oncancel={() => controller.requestClose()}
          onsubmit={async (value) => {
            if (value.kind === 'create') await controller.create(value.payload);
          }}
        />
      {:else if panel.kind === 'credential'}
        <ApiCredentialResult credential={panel.credential} onclose={() => controller.close()} />
      {:else if client}
        {#if panel.kind === 'details'}
          {#if controller.notice}<p class="mx-6 mt-5 text-sm text-success-fg" role="status">
              {controller.notice}
            </p>{/if}
          <ApiClientDetails
            {client}
            {now}
            busy={locked}
            onaction={(action) => controller.open(action)}
          />
        {:else if panel.kind === 'edit'}
          <ApiClientForm
            {client}
            busy={locked}
            ondirty={(dirty) => {
              controller.dirty = dirty;
            }}
            oncancel={() => controller.requestClose()}
            onsubmit={async (value) => {
              if (value.kind === 'edit' && client) await controller.update(client, value.payload);
            }}
          />
        {:else}
          <ApiKeyOperation
            kind={panel.kind}
            {client}
            prefix={panel.kind === 'revoke' ? panel.key.prefix : undefined}
            busy={locked}
            ondirty={() => {
              controller.dirty = true;
            }}
            oncancel={() => controller.requestClose()}
            onconfirm={async (value) => {
              if (!client) return;
              switch (panel.kind) {
                case 'rotate':
                  await controller.rotate(client.id, value.overlap);
                  break;
                case 'issue':
                  if (value.expiration) await controller.issue(client.id, value.expiration);
                  break;
                case 'revoke':
                  await controller.revoke(client.id, panel.key);
                  break;
                case 'toggle':
                  await controller.update(client, {
                    name: client.name,
                    description: client.description,
                    scopes: client.scopes,
                    enabled: client.disabled_at !== null,
                  });
                  break;
                default:
                  break;
              }
            }}
          />
        {/if}
      {:else}<p class="p-6 text-sm text-fg-muted">此调用方已不存在，请关闭面板后刷新列表。</p>{/if}
    </div>
  </ApiKeyPanel>
{/if}

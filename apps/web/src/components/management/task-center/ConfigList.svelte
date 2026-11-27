<script lang="ts">
  import { IconCopy, IconEdit } from '@tabler/icons-svelte';

  import type { RequestConfig } from '@/api/task-center/configuration.types';
  import { kindLabels } from '@/application/task-center/model';
  import type { TasksController } from '@/application/task-center/tasks.controller.svelte';

  import { buttonClass } from './styles';
  let {
    controller,
    system,
    onedit,
  }: {
    controller: TasksController;
    system: boolean;
    onedit: (value: RequestConfig, copy?: boolean) => void;
  } = $props();
  let archived = $state(false);
</script>

<div class="grid gap-4">
  <label class="flex min-h-11 items-center gap-3 text-sm"
    ><input type="checkbox" bind:checked={archived} />显示已归档配置</label
  >
  {#each controller.configs.filter((c) => archived || !c.archived) as config (config.id)}
    {@const permitted = system || config.parameters.signed}
    <article class="grid gap-3 border-b border-line py-4">
      <div class="flex flex-wrap justify-between gap-3">
        <div class="grid min-w-0 gap-2">
          <h2 class="font-semibold break-all">{config.name}</h2>
          <p class="text-sm text-fg-muted">
            {kindLabels[config.purpose]} · {config.archived
              ? '已归档'
              : config.enabled
                ? '已启用'
                : '已停用'} · {config.parameters.signed ? '签名开启' : '未签名'}
          </p>
          {#if config.verification_default}<p class="text-sm text-tint-fg">当前认证默认配置</p>{/if}
          <p class="text-sm break-all">
            {config.parameters.user_agent} · {config.parameters.timeout_seconds} 秒 · 间隔 {config
              .parameters.interval_ms} 毫秒
          </p>
        </div>
        <div class="flex flex-wrap items-start gap-2">
          <button
            class={buttonClass}
            disabled={controller.busy || !permitted}
            onclick={() => onedit(config)}><IconEdit size={16} />编辑</button
          >
          <button
            class={buttonClass}
            disabled={controller.busy || !permitted}
            title="复制配置"
            aria-label={`复制 ${config.name}`}
            onclick={() => onedit(config, true)}><IconCopy size={16} /></button
          >
          <button
            class={buttonClass}
            disabled={controller.busy || !permitted || config.archived}
            onclick={() =>
              controller.saveConfig({ ...config, enabled: !config.enabled }, config.id)}
            >{config.enabled ? '停用' : '启用'}</button
          >
          <button
            class={buttonClass}
            disabled={controller.busy || !permitted}
            onclick={() =>
              controller.saveConfig(
                { ...config, enabled: false, archived: !config.archived },
                config.id,
              )}>{config.archived ? '取消归档' : '归档'}</button
          >
          {#if config.purpose === 'verification' && !config.verification_default}<button
              class={buttonClass}
              disabled={controller.busy ||
                !config.enabled ||
                config.archived ||
                !config.parameters.signed}
              onclick={() => controller.bindVerification(config.id)}>设为认证默认</button
            >{/if}
        </div>
      </div>
      <details class="text-sm">
        <summary class="cursor-pointer text-fg-muted"
          >引用调度（{config.references.length}）</summary
        >
        <ul class="mt-2 grid gap-1">
          {#each config.references as id (id)}<li class="break-all">
              {controller.schedules.find((s) => s.id === id)?.name ?? id}
            </li>{:else}<li>暂无引用。</li>{/each}
        </ul>
      </details>
    </article>
  {:else}<p class="py-8 text-sm text-fg-muted">暂无请求配置。</p>{/each}
</div>

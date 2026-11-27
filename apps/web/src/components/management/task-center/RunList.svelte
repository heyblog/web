<script lang="ts">
  import type { Control, TaskQuery } from '@/api/task-center/tasks.types';
  import { runStates } from '@/api/task-center/tasks.types';
  import {
    actionLabels,
    date,
    kindLabels,
    sourceLabels,
    state as stateLabel,
  } from '@/application/task-center/model';
  import type { TasksController } from '@/application/task-center/tasks.controller.svelte';

  import Pagination from './Pagination.svelte';
  import { buttonClass, fieldClass, inputClass } from './styles';
  let { controller }: { controller: TasksController } = $props();
  let kind = $state(''),
    status = $state(''),
    source = $state(''),
    schedule = $state('');
  let from = $state(''),
    to = $state(''),
    archived = $state('false');
  let batch = $state<Control>('pause');
  function filter(event: SubmitEvent) {
    event.preventDefault();
    const query: TaskQuery = {
      kind,
      state: status,
      source,
      schedule,
      archived,
      from: from ? new Date(from).toISOString() : '',
      to: to ? new Date(to).toISOString() : '',
    };
    void controller.filter(query);
  }
</script>

<div class="grid min-w-0 gap-6">
  <form class="grid gap-4 sm:grid-cols-3" onsubmit={filter}>
    <label class={fieldClass}
      >类型<select class={inputClass} bind:value={kind}
        ><option value="">全部类型</option
        >{#each Object.entries(kindLabels) as [value, label] (value)}<option {value}>{label}</option
          >{/each}</select
      ></label
    >
    <label class={fieldClass}
      >状态<select class={inputClass} bind:value={status}
        ><option value="">全部状态</option>{#each runStates as value (value)}<option {value}
            >{stateLabel(value)}</option
          >{/each}</select
      ></label
    >
    <label class={fieldClass}
      >来源<select class={inputClass} bind:value={source}
        ><option value="">全部来源</option
        >{#each Object.entries(sourceLabels) as [value, label] (value)}<option {value}
            >{label}</option
          >{/each}</select
      ></label
    >
    <label class={fieldClass}
      >计划<select class={inputClass} bind:value={schedule}
        ><option value="">全部计划</option>{#each controller.schedules as item (item.id)}<option
            value={item.id}>{item.name}</option
          >{/each}</select
      ></label
    >
    <label class={fieldClass}
      >开始时间<input class={inputClass} type="datetime-local" bind:value={from} /></label
    >
    <label class={fieldClass}
      >结束时间<input class={inputClass} type="datetime-local" bind:value={to} /></label
    >
    <label class={fieldClass}
      >归档状态<select class={inputClass} bind:value={archived}
        ><option value="false">未归档</option><option value="true">已归档</option><option value=""
          >全部</option
        ></select
      ></label
    >
    <button class={`${buttonClass} self-end`} disabled={controller.busy}>筛选</button>
  </form>
  <div class="flex flex-wrap items-center gap-3">
    <label class="flex min-h-11 items-center gap-2 text-sm"
      ><input
        type="checkbox"
        checked={controller.runs.length > 0 &&
          controller.runs.every((r) => controller.checked.includes(r.id))}
        onchange={(e) => {
          controller.checked = e.currentTarget.checked ? controller.runs.map((r) => r.id) : [];
        }}
      />本页全选</label
    >
    <label class="sr-only" for="task-batch-action">批量操作</label>
    <select id="task-batch-action" class={`${inputClass} w-auto`} bind:value={batch}>
      {#each ['pause', 'resume', 'cancel', 'archive', 'unarchive'] as value (value)}<option {value}
          >{actionLabels[value]}</option
        >{/each}
    </select>
    <button
      class={buttonClass}
      disabled={controller.busy || !controller.checked.length}
      onclick={() => {
        if (
          window.confirm(
            `对已选 ${controller.checked.length} 个运行执行「${actionLabels[batch]}」？`,
          )
        )
          void controller.batch(batch);
      }}>应用（{controller.checked.length}）</button
    >
  </div>
  {#if controller.batchResults.length}<details class="text-sm" open>
      <summary>批量结果</summary>
      <ul class="mt-3 grid gap-2">
        {#each controller.batchResults as result (result.id)}<li class="break-all">
            {result.id} · {result.code === 'accepted'
              ? stateLabel(result.state)
              : '状态冲突，请刷新'}
          </li>{/each}
      </ul>
    </details>{/if}
  <div class="grid">
    {#each controller.runs as run (run.id)}
      <article
        class="flex min-w-0 flex-wrap items-start justify-between gap-4 border-b border-line py-4"
      >
        <div class="flex min-w-0 gap-3">
          <label class="flex min-h-11 shrink-0 items-start pt-1"
            ><span class="sr-only">选择运行 {run.id}</span><input
              type="checkbox"
              value={run.id}
              bind:group={controller.checked}
            /></label
          >
          <div class="grid min-w-0 gap-2">
            <h2 class="font-semibold">
              {kindLabels[run.kind]} · {stateLabel(run.state)}{run.archived ? ' · 已归档' : ''}
            </h2>
            <p class="text-sm text-fg-muted">{sourceLabels[run.source]} · {date(run.created_at)}</p>
            <p class="text-xs break-all text-fg-muted">{run.id}</p>
            <p class="text-sm">
              成功 {run.counts.succeeded} · 失败 {run.counts.failed} · 跳过 {run.counts.skipped} · 取消
              {run.counts.cancelled} · 待处理 {run.counts.pending}
            </p>
          </div>
        </div>
        <button
          class={buttonClass}
          disabled={controller.busy}
          onclick={() => controller.select(run)}>查看详情</button
        >
      </article>
    {:else}<p class="py-8 text-sm text-fg-muted">暂无符合筛选条件的运行。</p>{/each}
  </div>
  <Pagination
    page={controller.runPage.page}
    total={controller.runPage.total}
    busy={controller.busy}
    onchange={(p) => {
      void controller.page(p);
    }}
  />
</div>

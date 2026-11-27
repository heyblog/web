<script lang="ts">
  import { IconDownload, IconX } from '@tabler/icons-svelte';

  import type { Run } from '@/api/task-center/tasks.types';
  import {
    configurationRows,
    date,
    errorLabels,
    kindLabels,
    reason,
    sourceLabels,
    state as stateLabel,
  } from '@/application/task-center/model';
  import type { TasksController } from '@/application/task-center/tasks.controller.svelte';

  import Pagination from './Pagination.svelte';
  import RunActions from './RunActions.svelte';
  import RunHistory from './RunHistory.svelte';
  import { buttonClass, inputClass } from './styles';
  let {
    run,
    controller,
    system,
    oncopy,
    onclose,
  }: {
    run: Run;
    controller: TasksController;
    system: boolean;
    oncopy: () => void;
    onclose: () => void;
  } = $props();
  let dialog: HTMLDialogElement;
  $effect(() => {
    const previous = document.activeElement;
    dialog.showModal();
    return () => {
      dialog.close();
      if (previous instanceof HTMLElement && previous.isConnected) previous.focus();
    };
  });
  const counts = $derived(run.counts);
</script>

<dialog
  bind:this={dialog}
  aria-labelledby="run-heading"
  {onclose}
  class="fixed inset-y-0 right-0 left-auto m-0 h-dvh max-h-dvh w-full max-w-full border-0 border-l border-line bg-surface p-0 text-fg shadow-md sm:w-120"
>
  <div class="flex h-full min-h-0 flex-col">
    <header class="flex shrink-0 items-start justify-between gap-3 border-b border-line p-6">
      <div class="grid min-w-0 gap-2">
        <h2 id="run-heading" class="text-xl font-semibold">
          {kindLabels[run.kind]} · {stateLabel(run.state)}
        </h2>
        <p class="text-xs break-all text-fg-muted">{run.id}</p>
      </div>
      <button class={buttonClass} title="关闭详情" aria-label="关闭详情" onclick={onclose}
        ><IconX size={16} /></button
      >
    </header>
    <div class="grid min-h-0 content-start gap-6 overflow-y-auto p-6">
      <RunActions
        {run}
        busy={controller.busy}
        {system}
        oncontrol={(action) => {
          void controller.control(run, action);
        }}
        onretry={(selection) => {
          void controller.retry(run, selection);
        }}
        oncleanup={(confirmation) => {
          void controller.cleanup(run, confirmation);
        }}
        {oncopy}
      />
      {#if controller.error}<p role="alert" class="text-sm text-danger-fg">
          {controller.error}
        </p>{/if}
      {#if controller.notice}<p role="status" class="text-sm text-success-fg">
          {controller.notice}
        </p>{/if}
      {#if controller.refreshError}<p role="alert" class="text-sm text-warning-fg">
          详情暂未刷新，请刷新确认最新状态。
        </p>{/if}
      <dl class="grid gap-4 text-sm sm:grid-cols-2">
        {#each [['来源', sourceLabels[run.source]], ['创建时间', date(run.created_at)], ['首次执行', date(run.started_at)], ['结束时间', date(run.finished_at)], ['排队耗时', `${Math.round(run.queue_ms / 1000)} 秒`], ['执行耗时', `${Math.round(run.elapsed_ms / 1000)} / ${run.timeout_seconds} 秒`], ['完全暂停', `${Math.round(run.paused_ms / 1000)} 秒`], ['重试序号', String(run.retry_number)]] as [label, value] (label)}<div
          >
            <dt class="text-fg-muted">{label}</dt>
            <dd>{value}</dd>
          </div>{/each}
      </dl>
      <p class="text-sm">
        成功 {counts.succeeded} · 失败 {counts.failed} · 跳过 {counts.skipped} · 取消 {counts.cancelled}
        · 待处理 {counts.pending}
      </p>
      {#if run.reason}<p class="text-sm text-warning-fg">{reason(run.reason)}</p>{/if}
      {#if run.parent_run_id || (run.root_run_id && run.root_run_id !== run.id)}
        <div class="flex flex-wrap gap-2 text-sm">
          {#if run.parent_run_id}<button
              class={buttonClass}
              disabled={controller.busy}
              onclick={() => {
                if (run.parent_run_id) void controller.selectID(run.parent_run_id);
              }}>上一运行</button
            >{/if}
          {#if run.root_run_id && run.root_run_id !== run.id}<button
              class={buttonClass}
              disabled={controller.busy}
              onclick={() => {
                if (run.root_run_id) void controller.selectID(run.root_run_id);
              }}>最初运行</button
            >{/if}
        </div>
      {/if}
      {#if run.snapshot}<details class="border-y border-line py-4">
          <summary class="cursor-pointer text-sm font-medium">参数快照</summary>
          <dl class="mt-4 grid gap-3 text-sm">
            {#each configurationRows(run.snapshot) as [label, value] (label)}
              <div class="min-w-0">
                <dt class="text-fg-muted">{label}</dt>
                <dd class="break-all whitespace-pre-wrap">{value}</dd>
              </div>
            {/each}
          </dl>
        </details>{:else}<p class="text-sm text-fg-muted">详细参数已清理。</p>{/if}
      <div class="flex flex-wrap gap-2">
        <button
          class={buttonClass}
          disabled={controller.busy}
          onclick={() => controller.download('csv')}><IconDownload size={16} />目标 CSV</button
        >
        <button
          class={buttonClass}
          disabled={controller.busy}
          onclick={() => controller.download('json')}><IconDownload size={16} />运行 JSON</button
        >
      </div>
      <label class="grid gap-2 text-sm"
        >目标状态<select
          class={inputClass}
          bind:value={controller.details.targetState}
          onchange={() => {
            void controller.detailPage('targets', 1);
          }}
        >
          <option value="">全部目标</option>
          {#each ['pending', 'running', 'retry_wait', 'succeeded', 'partial_failed', 'failed', 'timed_out', 'skipped', 'cancelled'] as status (status)}<option
              value={status}>{stateLabel(status)}</option
            >{/each}
        </select></label
      >
      {#each controller.details.targetPage.items as target (target.id)}
        <article class="grid min-w-0 gap-3 border-t border-line pt-4 text-sm">
          <div class="grid gap-1">
            <strong class="break-all">{target.address || target.id}</strong><span
              class="text-fg-muted">{stateLabel(target.state)}</span
            >
          </div>
          <p class="text-xs text-fg-muted">
            采样 {date(target.sample_started_at)} · 故障恢复 {target.recoveries}
          </p>
          {#if target.reason}<p class="text-warning-fg">{reason(target.reason)}</p>{/if}
          {#if target.result}
            <p>
              业务结果：{target.result.business_valid ? '有效' : '未通过'}{target.result.fallback
                ? ' · 已使用备用平台'
                : ''}
            </p>
            {#each target.result.paths as path, index (`${path.platform}-${index}`)}
              <div class="grid gap-2 border-l border-line pl-3">
                <p>
                  {path.platform === 'EO' ? '大陆 · EdgeOne' : '境外 · Cloudflare'} · {path.outcome ===
                  'OK'
                    ? '访问正常'
                    : path.outcome === 'FAIL'
                      ? '访问失败'
                      : '无法判定'}
                </p>
                <p class="text-xs text-fg-muted">
                  HTTP {path.http_status || '—'} · {path.duration_ms} 毫秒 · {path.requests} 次 · {date(
                    path.observed_at,
                  )}
                </p>
                {#if path.error_kind}<p class="text-warning-fg">
                    {errorLabels[path.error_kind] ?? '执行异常'}
                  </p>{/if}
                {#if path.redirects.length}<details>
                    <summary class="cursor-pointer">重定向（{path.redirects.length}）</summary>
                    <ol class="mt-2 grid gap-1 break-all">
                      {#each path.redirects as address, i (`${i}-${address}`)}<li>
                          {address}
                        </li>{/each}
                    </ol>
                  </details>{/if}
                {#if path.node.colo}<p class="text-xs text-fg-muted">节点 {path.node.colo}</p>{/if}
              </div>
            {/each}
          {:else}<p class="text-fg-muted">{run.cleaned_at ? '详情已清理。' : '尚无结果。'}</p>{/if}
        </article>
      {:else}<p class="text-sm text-fg-muted">暂无符合筛选条件的目标。</p>{/each}
      <Pagination
        page={controller.details.targetPage.page}
        total={controller.details.targetPage.total}
        busy={controller.busy}
        onchange={(p) => {
          void controller.detailPage('targets', p);
        }}
      />
      <RunHistory {controller} />
    </div>
  </div>
</dialog>

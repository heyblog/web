<script lang="ts">
  import { date, kindLabels, reason, state as stateLabel } from '@/application/task-center/model';
  import type { TasksController } from '@/application/task-center/tasks.controller.svelte';

  import { buttonClass, fieldClass, inputClass } from './styles';
  let {
    controller,
    view,
    system,
  }: {
    controller: TasksController;
    view: 'overview' | 'workers' | 'stats';
    system: boolean;
  } = $props();
  let platform = $state('');
  let kind = $state('');
  let from = $state('');
  let to = $state('');
  const stats = $derived(controller.stats.filter((s) => !platform || s.platform === platform));
</script>

<div class="grid min-w-0 gap-6">
  {#if view === 'overview'}
    <dl class="grid grid-cols-2 gap-6 border-b border-line pb-6 sm:grid-cols-4">
      {#each Object.entries(controller.overview?.states ?? {}) as [value, count] (value)}
        <div class="grid gap-2">
          <dt class="text-sm text-fg-muted">{stateLabel(value)}</dt>
          <dd class="text-2xl font-semibold">{count}</dd>
        </div>
      {:else}<p class="text-sm text-fg-muted">暂无运行。</p>{/each}
    </dl>
    <p class="text-sm">
      在线 worker {controller.workers.filter((w) => w.online).length} · 活动目标 {controller.workers.reduce(
        (sum, w) => sum + w.active,
        0,
      )}
    </p>
    <h2 class="text-lg font-semibold">平台状态</h2>
    {#each controller.overview?.backoff ?? [] as backoff (backoff.platform)}
      <div class="flex flex-wrap justify-between gap-3 border-b border-line pb-3 text-sm">
        <strong>{backoff.platform}</strong>
        <span>{backoff.retry_at ? `等待恢复 · ${date(backoff.retry_at)}` : '可请求'}</span>
        {#if backoff.reason}<span class="text-warning-fg">{reason(backoff.reason)}</span>{/if}
      </div>
    {/each}
    <h2 class="text-lg font-semibold">近期异常</h2>
    {#each controller.overview?.recent ?? [] as run (run.id)}
      <div
        class="flex flex-wrap items-center justify-between gap-3 border-b border-line pb-3 text-sm"
      >
        <p>{kindLabels[run.kind]} · {date(run.created_at)} · {stateLabel(run.state)}</p>
        <button
          class={buttonClass}
          disabled={controller.busy}
          onclick={() => controller.select(run)}>查看详情</button
        >
      </div>
    {:else}<p class="text-sm text-fg-muted">暂无近期异常。</p>{/each}
  {:else if view === 'workers'}
    {#each controller.workers as worker (worker.instance_id)}
      <article class="grid gap-3 border-b border-line py-4 text-sm">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="grid min-w-0 gap-2">
            <h2 class="font-medium break-all">{worker.instance_id}</h2>
            <p>
              {worker.online ? '在线' : '离线'} · {worker.draining ? '停止领取中' : '正常领取'} · 活动目标
              {worker.active}
            </p>
          </div>
          {#if system}<button
              class={buttonClass}
              disabled={controller.busy || !worker.online}
              onclick={() =>
                controller.worker(worker.instance_id, worker.draining ? 'resume' : 'drain')}
              >{worker.draining ? '恢复领取' : '停止领取'}</button
            >{/if}
        </div>
        <p class="text-fg-muted">
          版本 {worker.version} · 注册 {date(worker.registered_at)} · 心跳 {date(
            worker.heartbeat_at,
          )}
        </p>
      </article>
    {:else}<p class="py-8 text-sm text-fg-muted">暂无 worker 实例。</p>{/each}
  {:else}
    <form
      class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4"
      onsubmit={(event) => {
        event.preventDefault();
        void controller.filterStats({
          kind,
          from: from ? new Date(from).toISOString() : '',
          to: to ? new Date(to).toISOString() : '',
        });
      }}
    >
      <label class={fieldClass}
        >平台<select class={inputClass} bind:value={platform}>
          <option value="">全部平台</option><option value="CF">Cloudflare</option><option value="EO"
            >EdgeOne</option
          >
        </select></label
      >
      <label class={fieldClass}
        >类型<select class={inputClass} bind:value={kind}>
          <option value="">全部类型</option>
          {#each Object.entries(kindLabels) as [value, label] (value)}<option {value}
              >{label}</option
            >{/each}
        </select></label
      >
      <label class={fieldClass}
        >开始时间<input
          class={inputClass}
          type="datetime-local"
          bind:value={from}
          max={to || undefined}
        /></label
      >
      <label class={fieldClass}
        >结束时间<input
          class={inputClass}
          type="datetime-local"
          bind:value={to}
          min={from || undefined}
        /></label
      >
      <button class={`${buttonClass} w-fit`} disabled={controller.busy}>筛选统计</button>
    </form>
    <div class="overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-line text-xs text-fg-muted"
          ><tr
            >{#each ['平台 / 类型', '授权', '报告发送', '收到结果', '成功', '失败', '未知', '耗时(ms)', '逻辑重试次数', '重定向跳数', '备用目标数'] as label (label)}<th
                class="px-4 py-3 font-medium whitespace-nowrap">{label}</th
              >{/each}</tr
          ></thead
        >
        <tbody>
          {#each stats as stat (`${stat.platform}-${stat.kind}`)}
            <tr class="border-b border-line even:bg-subtle/50">
              <th class="px-4 py-3 font-medium whitespace-nowrap"
                >{stat.platform} / {kindLabels[stat.kind] ?? '历史请求'}</th
              >
              {#each [stat.authorized, stat.sent, stat.received, stat.succeeded, stat.failed, stat.unknown, stat.duration_ms, stat.retries, stat.redirects, stat.fallbacks] as count, i (i)}<td
                  class="px-4 py-3 tabular-nums">{count}</td
                >{/each}
            </tr>
          {:else}<tr><td colspan="11" class="py-8 text-fg-muted">暂无请求统计。</td></tr>{/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

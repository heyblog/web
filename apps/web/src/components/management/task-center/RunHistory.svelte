<script lang="ts">
  import { date, state as stateLabel } from '@/application/task-center/model';
  import type { TasksController } from '@/application/task-center/tasks.controller.svelte';

  import Pagination from './Pagination.svelte';
  import { buttonClass } from './styles';
  let { controller }: { controller: TasksController } = $props();
  let tab = $state<'attempts' | 'events'>('attempts');
  const eventLabels: Readonly<Record<string, string>> = {
    run_created: '创建运行',
    run_running: '开始运行',
    run_succeeded: '执行成功',
    run_failed: '执行失败',
    run_partial_failed: '执行不完整',
    run_timed_out: '运行超时',
    run_paused: '已暂停',
    run_cancelled: '已取消',
    pause: '请求暂停',
    resume: '恢复运行',
    cancel: '请求取消',
    archive: '归档',
    unarchive: '取消归档',
    cleanup: '清理详情',
  };
</script>

<div class="grid gap-4 border-t border-line pt-6">
  <div class="flex gap-2" aria-label="执行历史视图">
    <button
      class={buttonClass}
      aria-pressed={tab === 'attempts'}
      onclick={() => {
        tab = 'attempts';
      }}>请求尝试</button
    >
    <button
      class={buttonClass}
      aria-pressed={tab === 'events'}
      onclick={() => {
        tab = 'events';
      }}>事件时间线</button
    >
  </div>
  {#if tab === 'attempts'}
    {#each controller.details.attemptPage.items as attempt (attempt.id)}
      <article class="grid min-w-0 gap-2 border-b border-line pb-4 text-sm">
        <p class="font-medium">
          {attempt.platform} · {stateLabel(attempt.state)} · {attempt.signed ? '已签名' : '未签名'}
        </p>
        <p class="break-all">{attempt.method} {attempt.url || '地址详情已清理'}</p>
        <p class="text-fg-muted">
          重试 {attempt.retry} · 重定向 {attempt.hop} · {attempt.accepted
            ? '结果已接受'
            : '尚未接受结果'}
        </p>
        <dl class="grid gap-2 text-xs text-fg-muted">
          <div>
            <dt>请求 ID</dt>
            <dd class="break-all">{attempt.id}</dd>
          </div>
          {#if attempt.parent_id}<div>
              <dt>上一跳</dt>
              <dd class="break-all">{attempt.parent_id}</dd>
            </div>{/if}
          <div>
            <dt>授权时间</dt>
            <dd>{date(attempt.created_at)}</dd>
          </div>
          <div>
            <dt>发送 / 接收</dt>
            <dd>{date(attempt.sent_at)} / {date(attempt.received_at)}</dd>
          </div>
        </dl>
      </article>
    {:else}<p class="text-sm text-fg-muted">暂无请求尝试。</p>{/each}
    <Pagination
      page={controller.details.attemptPage.page}
      total={controller.details.attemptPage.total}
      busy={controller.busy}
      onchange={(p) => {
        void controller.detailPage('attempts', p);
      }}
    />
  {:else}
    <ol class="grid gap-4">
      {#each controller.details.eventPage.items as event (event.id)}
        <li class="grid gap-1 border-b border-line pb-3 text-sm">
          <p>
            {eventLabels[event.action] ?? '任务状态更新'} · {event.outcome === 'accepted'
              ? '已受理'
              : '已记录'}
          </p>
          <time class="text-xs text-fg-muted" datetime={event.created_at}
            >{date(event.created_at)}</time
          >
          <p class="text-xs break-all text-fg-muted">
            {event.actor_id ? `操作者 ${event.actor_id}` : '系统'}
          </p>
        </li>
      {:else}<li class="text-sm text-fg-muted">暂无事件。</li>{/each}
    </ol>
    <Pagination
      page={controller.details.eventPage.page}
      total={controller.details.eventPage.total}
      busy={controller.busy}
      onchange={(p) => {
        void controller.detailPage('events', p);
      }}
    />
  {/if}
</div>

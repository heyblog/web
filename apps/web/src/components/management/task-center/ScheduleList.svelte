<script lang="ts">
  import { IconCopy, IconEdit, IconPlayerPlay } from '@tabler/icons-svelte';

  import type { Schedule } from '@/api/task-center/configuration.types';
  import { date, kindLabels } from '@/application/task-center/model';
  import type { TasksController } from '@/application/task-center/tasks.controller.svelte';

  import { buttonClass } from './styles';
  let {
    controller,
    system,
    onedit,
  }: {
    controller: TasksController;
    system: boolean;
    onedit: (value: Schedule, copy?: boolean) => void;
  } = $props();
  let archived = $state(false);
</script>

<div class="grid gap-4">
  <label class="flex min-h-11 items-center gap-3 text-sm"
    ><input type="checkbox" bind:checked={archived} />显示已归档调度</label
  >
  {#each controller.schedules.filter((s) => archived || !s.archived) as schedule (schedule.id)}
    {@const config = controller.configs.find(
      (c) => c.id === schedule.configuration.request_config_id,
    )}
    {@const permitted = system || config?.parameters.signed === true}
    <article class="grid gap-4 border-b border-line py-4">
      <div class="flex flex-wrap justify-between gap-3">
        <div class="grid min-w-0 gap-2">
          <h2 class="font-semibold break-all">{schedule.name}</h2>
          <p class="text-sm text-fg-muted">
            {kindLabels[schedule.kind]} · {schedule.archived
              ? '已归档'
              : schedule.enabled
                ? '已启用'
                : '已停用'} · {schedule.configuration.timezone}
          </p>
          <p class="text-sm">请求配置：{config?.name ?? '配置不可用'}</p>
        </div>
        <div class="flex flex-wrap items-start gap-2">
          <button
            class={buttonClass}
            disabled={controller.busy || !permitted}
            onclick={() => onedit(schedule)}><IconEdit size={16} />编辑</button
          >
          <button
            class={buttonClass}
            disabled={controller.busy || !permitted}
            title="复制调度"
            aria-label={`复制 ${schedule.name}`}
            onclick={() => onedit(schedule, true)}><IconCopy size={16} /></button
          >
          <button
            class={buttonClass}
            disabled={controller.busy || !permitted || schedule.archived}
            onclick={() => controller.toggle(schedule)}>{schedule.enabled ? '停用' : '启用'}</button
          >
          <button
            class={buttonClass}
            disabled={controller.busy || !permitted || schedule.archived}
            onclick={() => {
              if (window.confirm(`立即运行「${schedule.name}」？`)) void controller.run(schedule);
            }}><IconPlayerPlay size={16} />立即运行</button
          >
          <button
            class={buttonClass}
            disabled={controller.busy || !permitted}
            onclick={() => controller.archiveSchedule(schedule)}
            >{schedule.archived ? '取消归档' : '归档'}</button
          >
        </div>
      </div>
      <p class="text-sm text-fg-muted">
        {schedule.configuration.mode === 'cron'
          ? schedule.configuration.cron
          : `每 ${schedule.configuration.interval_seconds} 秒`} · 下次执行 {date(schedule.next_at)}
      </p>
    </article>
  {:else}<p class="py-8 text-sm text-fg-muted">暂无调度计划。</p>{/each}
</div>

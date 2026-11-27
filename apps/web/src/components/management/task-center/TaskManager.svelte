<script lang="ts">
  import { IconPlus, IconRefresh } from '@tabler/icons-svelte';
  import { onMount } from 'svelte';

  import type {
    RequestConfig,
    RequestConfigInput,
    Schedule,
    ScheduleInput,
  } from '@/api/task-center/configuration.types';
  import { protectFormDraft } from '@/application/navigation/form-navigation.browser';
  import { newSchedule } from '@/application/task-center/model';
  import { TasksController } from '@/application/task-center/tasks.controller.svelte';

  import ConfigList from './ConfigList.svelte';
  import RequestConfigForm from './RequestConfigForm.svelte';
  import RunDetails from './RunDetails.svelte';
  import RunList from './RunList.svelte';
  import ScheduleForm from './ScheduleForm.svelte';
  import ScheduleList from './ScheduleList.svelte';
  import { buttonClass, primaryClass } from './styles';
  import TaskOverview from './TaskOverview.svelte';
  let { system = false }: { system?: boolean } = $props();
  const controller = new TasksController();
  const tabs = [
    { id: 'overview', label: '概览' },
    { id: 'schedules', label: '调度计划' },
    { id: 'configs', label: '请求配置' },
    { id: 'runs', label: '运行记录' },
    { id: 'workers', label: 'Worker' },
    { id: 'stats', label: '运行统计' },
  ] as const;
  let tab = $state<(typeof tabs)[number]['id']>('overview');
  let editor = $state<{ id?: string; value: ScheduleInput; manual: boolean; key: string } | null>(
    null,
  );
  let configEditor = $state<{ id?: string; value: RequestConfigInput } | null>(null);
  function discard() {
    return !(editor || configEditor) || window.confirm('放弃尚未保存的修改？');
  }
  function edit(schedule?: Schedule, copy = false, manual = false) {
    if (!discard()) return;
    configEditor = null;
    editor = {
      ...(schedule && !copy ? { id: schedule.id } : {}),
      value: schedule
        ? {
            ...schedule,
            ...(copy
              ? { name: `${schedule.name} 副本`, revision: 0, enabled: false, archived: false }
              : {}),
          }
        : newSchedule(),
      manual,
      key: crypto.randomUUID(),
    };
  }
  function editConfig(config?: RequestConfig, copy = false) {
    if (!discard()) return;
    editor = null;
    configEditor = {
      ...(config && !copy ? { id: config.id } : {}),
      value: config
        ? {
            ...config,
            ...(copy
              ? { name: `${config.name} 副本`, revision: 0, enabled: false, archived: false }
              : {}),
          }
        : {
            name: '',
            purpose: 'monitor',
            revision: 0,
            enabled: false,
            archived: false,
            parameters: newSchedule().configuration,
          },
    };
  }
  onMount(() => {
    void controller.refresh();
    const stopPolling = controller.startPolling();
    const stopGuard = protectFormDraft(() => ({
      dirty: editor !== null || configEditor !== null,
      busy: controller.busy,
    }));
    return () => {
      stopPolling();
      stopGuard();
    };
  });
</script>

<div class="grid min-w-0 gap-6">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <nav class="flex flex-wrap gap-2" aria-label="任务中心视图">
      {#each tabs as item (item.id)}<button
          class={`${buttonClass} ${tab === item.id ? 'bg-subtle text-fg' : 'text-fg-muted'}`}
          aria-pressed={tab === item.id}
          onclick={() => {
            if (discard()) {
              editor = null;
              configEditor = null;
              tab = item.id;
            }
          }}>{item.label}</button
        >{/each}
    </nav>
    <button
      class={buttonClass}
      title="刷新"
      aria-label="刷新任务中心"
      disabled={controller.busy}
      onclick={() => controller.refresh()}><IconRefresh size={16} /></button
    >
  </div>
  {#if controller.error}<p class="rounded-md bg-danger-bg p-4 text-sm text-danger-fg" role="alert">
      {controller.error}
    </p>{/if}
  {#if controller.notice}<p class="text-sm text-success-fg" role="status">
      {controller.notice}
    </p>{/if}
  {#if controller.refreshError}<p class="text-sm text-warning-fg" role="alert">
      列表暂未刷新，已受理的操作仍然有效。请刷新确认最新状态。
    </p>{/if}
  {#if controller.busy}<p class="text-sm text-fg-muted" role="status">正在处理…</p>{/if}
  {#if tab === 'schedules' || tab === 'runs'}
    {#if editor}
      {#key editor.key}<ScheduleForm
          initial={editor.value}
          configs={controller.configs}
          busy={controller.busy}
          {system}
          manual={editor.manual}
          oncancel={() => {
            if (discard()) editor = null;
          }}
          onsave={async (value) => {
            if (!editor) return;
            const accepted = editor.manual
              ? await controller.create(
                  { kind: value.kind, configuration: value.configuration },
                  editor.key,
                )
              : await controller.save(value, editor.id);
            if (accepted) editor = null;
          }}
        />{/key}
    {:else}
      <button
        class={`${primaryClass} w-fit`}
        disabled={controller.busy}
        onclick={() => edit(undefined, false, tab === 'runs')}
        ><IconPlus size={16} />{tab === 'runs' ? '新建手动任务' : '新建调度'}</button
      >
    {/if}
  {/if}
  {#if tab === 'schedules'}
    <ScheduleList {controller} {system} onedit={edit} />
  {:else if tab === 'configs'}
    {#if configEditor}
      {#key configEditor}<RequestConfigForm
          initial={configEditor.value}
          busy={controller.busy}
          {system}
          oncancel={() => {
            if (discard()) configEditor = null;
          }}
          onsave={async (value) => {
            if (await controller.saveConfig(value, configEditor?.id)) configEditor = null;
          }}
        />{/key}
    {:else}<button
        class={`${primaryClass} w-fit`}
        disabled={controller.busy}
        onclick={() => editConfig()}><IconPlus size={16} />新建请求配置</button
      >{/if}
    <ConfigList {controller} {system} onedit={editConfig} />
  {:else if tab === 'runs'}
    <RunList {controller} />
  {:else}<TaskOverview {controller} {system} view={tab} />{/if}
  {#if controller.details.selected}<RunDetails
      run={controller.details.selected}
      {controller}
      {system}
      onclose={() => {
        controller.details.selected = null;
      }}
      oncopy={() => {
        const run = controller.details.selected;
        if (!run?.snapshot || run.kind === 'verification') return;
        const value = { ...newSchedule(), kind: run.kind, configuration: run.snapshot };
        if (!discard()) return;
        controller.details.selected = null;
        configEditor = null;
        editor = { value, manual: true, key: crypto.randomUUID() };
        tab = 'runs';
      }}
    />{/if}
</div>

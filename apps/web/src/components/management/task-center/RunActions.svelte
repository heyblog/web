<script lang="ts">
  import {
    IconArchive,
    IconPlayerPause,
    IconPlayerPlay,
    IconPlayerStop,
    IconRepeat,
    IconRestore,
    IconTrash,
  } from '@tabler/icons-svelte';

  import type { Control, RetrySelection, Run } from '@/api/task-center/tasks.types';
  import { actionLabels } from '@/application/task-center/model';

  import { buttonClass, inputClass } from './styles';
  let {
    run,
    busy,
    system,
    oncontrol,
    onretry,
    oncleanup,
    oncopy,
  }: {
    run: Run;
    busy: boolean;
    system: boolean;
    oncontrol: (action: Control) => void;
    onretry: (selection: RetrySelection) => void;
    oncleanup: (confirmation: string) => void;
    oncopy: () => void;
  } = $props();
  let confirmation = $state('');
  let cleaning = $state(false);
  const icons = {
    pause: IconPlayerPause,
    resume: IconPlayerPlay,
    cancel: IconPlayerStop,
    archive: IconArchive,
    unarchive: IconRestore,
  };
  const actions = ['pause', 'resume', 'cancel', 'archive', 'unarchive'] as const;
  function control(action: Control) {
    if (
      action === 'cancel' &&
      !window.confirm('取消本次运行？已提交的结果会保留，已发出的请求可能仍会完成。')
    )
      return;
    oncontrol(action);
  }
</script>

<div class="grid gap-4">
  <div class="flex flex-wrap gap-2">
    {#each actions.filter((a) => run.allowed_actions.includes(a)) as action (action)}
      {@const Icon = icons[action]}
      <button class={buttonClass} disabled={busy} onclick={() => control(action)}
        ><Icon size={16} />{actionLabels[action]}</button
      >
    {/each}
    {#if run.kind !== 'verification' && run.snapshot && !run.cleaned_at && (system || run.snapshot.signed)}
      {#if run.allowed_actions.includes('retry')}
        <button
          class={buttonClass}
          disabled={busy}
          onclick={() => {
            if (window.confirm('沿用原参数，创建新的运行并重试失败或超时目标？')) onretry('failed');
          }}><IconRestore size={16} />重试失败目标</button
        >
        {#if run.state === 'cancelled'}<button
            class={buttonClass}
            disabled={busy}
            onclick={() => {
              if (window.confirm('沿用原参数，重新执行未完成目标？')) onretry('unfinished');
            }}>重跑未完成目标</button
          >{/if}
      {/if}
      {#if run.allowed_actions.includes('rerun')}
        <button
          class={buttonClass}
          disabled={busy}
          onclick={() => {
            if (window.confirm('沿用原参数，全部重新运行？')) onretry('all');
          }}><IconRepeat size={16} />全部重跑</button
        >
      {/if}
      <button class={buttonClass} disabled={busy} onclick={oncopy}>复制为新任务</button>
    {/if}
    {#if system && !run.cleaned_at && run.allowed_actions.includes('cleanup')}
      <button
        class={buttonClass}
        disabled={busy}
        onclick={() => {
          cleaning = !cleaning;
        }}><IconTrash size={16} />清理详情</button
      >
    {/if}
  </div>
  {#if cleaning}
    <form
      class="grid gap-3 bg-subtle p-4"
      onsubmit={(event) => {
        event.preventDefault();
        oncleanup(confirmation);
      }}
    >
      <p class="text-sm">清理后无法查看详细结果。检测样本、RSS 内容和累计统计会保留。</p>
      <label class="grid gap-2 text-sm"
        >输入运行 ID 确认<input
          class={`${inputClass} font-mono text-xs`}
          required
          bind:value={confirmation}
          autocomplete="off"
        /></label
      >
      <div class="flex gap-2">
        <button class={buttonClass} disabled={busy || confirmation !== run.id}>确认清理</button>
        <button
          type="button"
          class={buttonClass}
          onclick={() => {
            cleaning = false;
          }}>返回</button
        >
      </div>
    </form>
  {/if}
</div>

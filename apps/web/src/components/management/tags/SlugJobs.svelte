<script lang="ts">
  import { onMount } from 'svelte';

  import {
    applySlugJob,
    controlSlugJob,
    createSlugJob,
    editSlugJob,
    listSlugJobs,
    readSlugJob,
  } from '@/api/taxonomy/slug-jobs.browser';
  import type { SlugJob, SlugSelection } from '@/api/taxonomy/slug-jobs.types';
  import { taxonomyMessage } from '@/application/taxonomy/taxonomy.messages';

  import { inputClass, outlineClass, primaryClass } from './tags.styles';

  let {
    checked,
    query,
    isEnabled,
    onapplied,
    onediting,
  }: {
    checked: readonly string[];
    query: string;
    isEnabled?: boolean;
    onapplied: () => void | Promise<void>;
    onediting: (editing: boolean) => void;
  } = $props();
  let selection = $state<'invalid' | 'ids' | 'filter' | 'all'>('invalid');
  let jobs = $state<readonly SlugJob[]>([]);
  let job = $state<SlugJob | null>(null);
  let edits = $state<Record<string, string>>({});
  let selected = $state<string[]>([]);
  let busy = $state(false);
  let error = $state('');
  let confirm = $state(false);
  let active = false;
  let generation = 0;
  const statusText: Readonly<Record<string, string>> = {
    queued: '等待生成',
    running: '正在生成',
    paused: '已暂停',
    ready: '等待确认',
    cancelled: '已取消',
    completed: '已完成',
  };
  const stateText: Readonly<Record<string, string>> = {
    pending: '等待生成',
    running: '正在生成',
    ready: '待应用',
    needs_confirmation: '需确认含义',
    failed: '生成失败',
    stale: '标签已变化',
    applied: '已应用',
  };
  const dirty = $derived(
    job?.items.some(
      (item) => edits[item.tag_id] !== undefined && edits[item.tag_id] !== item.slug,
    ) ?? false,
  );
  $effect(() => onediting(dirty || busy));
  const readyIDs = $derived(
    job?.items.filter((item) => item.state === 'ready').map((item) => item.tag_id) ?? [],
  );
  function receive(value: SlugJob, reset = false): void {
    job = value;
    jobs = [value, ...jobs.filter((item) => item.id !== value.id)];
    if (reset) {
      edits = {};
      selected = value.items.filter((item) => item.state === 'ready').map((item) => item.tag_id);
      confirm = false;
    } else
      selected = selected.filter((id) =>
        value.items.some((item) => item.tag_id === id && item.state === 'ready'),
      );
  }
  onMount(() => {
    active = true;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    async function poll(): Promise<void> {
      if (!active) return;
      const token = generation;
      if (
        !busy &&
        !dirty &&
        !confirm &&
        job &&
        ['queued', 'running', 'paused'].includes(job.status)
      ) {
        const id = job.id;
        const result = await readSlugJob(id, controller.signal);
        if (active && token === generation && job?.id === id) {
          if (result.ok) receive(result.value);
          else error = taxonomyMessage(result.code);
        }
      }
      if (active) timer = setTimeout(() => void poll(), 3000);
    }
    void (async () => {
      const result = await listSlugJobs(controller.signal);
      if (!active) return;
      if (result.ok) {
        jobs = result.value;
        if (!job && result.value[0]) receive(result.value[0], true);
      } else error = taxonomyMessage(result.code);
      await poll();
    })();
    return () => {
      active = false;
      controller.abort();
      generation += 1;
      if (timer) clearTimeout(timer);
    };
  });
  async function create(): Promise<void> {
    if (busy) return;
    if (selection === 'ids' && (!checked.length || checked.length > 500)) {
      error = '请选择 1 至 500 个标签。';
      return;
    }
    if (dirty) {
      error = '请先保存当前候选修改。';
      return;
    }
    const input: SlugSelection =
      selection === 'ids'
        ? { kind: 'ids', ids: checked }
        : selection === 'filter'
          ? {
              kind: 'filter',
              filter: { query, ...(isEnabled === undefined ? {} : { is_enabled: isEnabled }) },
            }
          : { kind: selection };
    busy = true;
    generation += 1;
    error = '';
    const result = await createSlugJob(input);
    if (active) {
      if (result.ok) receive(result.value, true);
      else error = taxonomyMessage(result.code);
      busy = false;
    }
  }
  async function open(id: string): Promise<void> {
    if (busy || dirty) return;
    busy = true;
    generation += 1;
    error = '';
    const result = await readSlugJob(id);
    if (active) {
      if (result.ok) receive(result.value, true);
      else error = taxonomyMessage(result.code);
      busy = false;
    }
  }
  async function control(action: 'pause' | 'resume' | 'cancel' | 'retry'): Promise<void> {
    if (!job || busy || dirty) return;
    busy = true;
    generation += 1;
    error = '';
    const result = await controlSlugJob(job.id, action, job.revision);
    if (active) {
      if (result.ok) receive(result.value, true);
      else error = taxonomyMessage(result.code);
      busy = false;
    }
  }
  async function save(): Promise<void> {
    if (!job || busy || !dirty) return;
    const items = job.items
      .filter((item) => edits[item.tag_id] !== undefined && edits[item.tag_id] !== item.slug)
      .map((item) => ({ tag_id: item.tag_id, slug: edits[item.tag_id]?.trim() ?? '' }));
    if (
      items.some((item) => !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(item.slug) || item.slug.length > 128)
    ) {
      error = 'Slug 只能包含小写英文、数字和连字符。';
      return;
    }
    busy = true;
    generation += 1;
    error = '';
    const result = await editSlugJob(job.id, job.revision, items);
    if (active) {
      if (result.ok) receive(result.value, true);
      else error = taxonomyMessage(result.code);
      busy = false;
    }
  }
  async function apply(): Promise<void> {
    if (!job || busy || dirty || !selected.length || !confirm) return;
    busy = true;
    generation += 1;
    error = '';
    const result = await applySlugJob(job.id, job.revision, selected);
    if (active) {
      if (result.ok) {
        receive(result.value, true);
        await onapplied();
      } else {
        error = taxonomyMessage(result.code);
        confirm = false;
      }
      busy = false;
    }
  }
</script>

<svelte:window
  onbeforeunload={(event) => {
    if (dirty) {
      event.preventDefault();
      event.returnValue = '';
    }
  }}
/>
<section class="grid gap-5" aria-label="批量更新 slug" aria-busy={busy}>
  <div class="grid gap-4 rounded-md border border-line bg-surface p-5">
    <label class="grid gap-1.5 text-sm"
      >生成范围<select class={inputClass} bind:value={selection} disabled={busy || dirty}
        ><option value="invalid">旧格式或不规范的 slug</option><option value="ids"
          >字典中勾选的标签（{checked.length} 个）</option
        ><option value="filter">按名称与状态筛选</option><option value="all"
          >重新生成全部标签</option
        ></select
      ></label
    >
    {#if selection === 'filter'}<p class="text-sm text-fg-muted">
        名称或 slug：{query || '不限'}；状态：{isEnabled === undefined
          ? '全部'
          : isEnabled
            ? '已启用'
            : '已停用'}。
      </p>{/if}
    {#if selection === 'all'}<p class="text-sm text-warning-fg">
        将重新生成所有标签的候选 slug，确认应用后才会修改现有链接。
      </p>{/if}
    <p class="text-xs text-fg-muted">每次最多 500 个标签。生成后可编辑候选并选择应用。</p>
    <button class={primaryClass} type="button" disabled={busy || dirty} onclick={create}
      >生成预览</button
    >
  </div>
  {#if jobs.length}<label class="grid gap-1.5 text-sm"
      >生成任务<select
        class={inputClass}
        value={job?.id ?? ''}
        disabled={busy || dirty}
        onchange={(event) => open(event.currentTarget.value)}
        >{#each jobs as item (item.id)}<option value={item.id}
            >{statusText[item.status]} · {item.counts.total} 个标签 · {item.id.slice(-8)}</option
          >{/each}</select
      ></label
    >{/if}
  {#if job}
    <div class="grid gap-3">
      <p class="text-sm" role="status">
        {statusText[job.status]} · {job.counts.needs_confirmation} 个需确认 · {job.counts.ready} 个待应用
        · {job.counts.failed} 个失败 · {job.counts.applied} 个已应用 / 共 {job.counts.total} 个
      </p>
      {#if job.pause_code}<p class="text-sm text-warning-fg">
          {taxonomyMessage(job.pause_code)}{#if job.resume_after}
            可恢复时间：{job.resume_after}{/if}
        </p>{/if}
      <div class="flex flex-wrap gap-3">
        <button
          class={outlineClass}
          type="button"
          disabled={busy || dirty}
          onclick={() => job && open(job.id)}>刷新任务</button
        >
        {#if job.status === 'running' || job.status === 'queued'}<button
            class={outlineClass}
            type="button"
            disabled={busy || dirty}
            onclick={() => control('pause')}>暂停</button
          >{/if}
        {#if job.status === 'paused'}<button
            class={outlineClass}
            type="button"
            disabled={busy || dirty}
            onclick={() => control('resume')}>继续生成</button
          >{/if}
        {#if job.counts.failed > 0 && ['ready', 'paused'].includes(job.status)}<button
            class={outlineClass}
            type="button"
            disabled={busy || dirty}
            onclick={() => control('retry')}>重试失败项</button
          >{/if}
        {#if ['running', 'queued', 'paused', 'ready'].includes(job.status)}<button
            class={outlineClass}
            type="button"
            disabled={busy || dirty}
            onclick={() => control('cancel')}>取消任务</button
          >{/if}
      </div>
    </div>
    <div class="overflow-x-auto rounded-md border border-line bg-surface">
      <table class="w-full text-left text-sm">
        <caption class="sr-only">Slug 更新预览</caption><thead
          ><tr
            ><th class="p-3" scope="col">选择</th><th class="p-3" scope="col">标签与原 slug</th><th
              class="p-3"
              scope="col">候选 slug</th
            ><th class="p-3" scope="col">状态</th></tr
          ></thead
        ><tbody>
          {#each job.items as item (item.tag_id)}<tr class="border-t border-line"
              ><td class="p-3"
                ><input
                  type="checkbox"
                  bind:group={selected}
                  value={item.tag_id}
                  disabled={busy || item.state !== 'ready' || job.status !== 'ready'}
                  aria-label={`应用 ${item.name}`}
                  onchange={() => (confirm = false)}
                /></td
              ><th class="p-3 font-medium" scope="row"
                >{item.name}<span class="mt-1 block text-xs font-normal wrap-anywhere text-fg-muted"
                  >{item.original_slug}</span
                ></th
              ><td class="p-3"
                ><input
                  class={inputClass}
                  value={edits[item.tag_id] ?? item.slug}
                  aria-label={`${item.name} 的候选 slug`}
                  maxlength="128"
                  disabled={busy ||
                    !['ready', 'needs_confirmation'].includes(item.state) ||
                    job.status !== 'ready'}
                  oninput={(event) => {
                    edits[item.tag_id] = event.currentTarget.value;
                    confirm = false;
                  }}
                /></td
              ><td class="p-3"
                >{stateText[item.state]}{#if item.conflicts.length}<p
                    class="mt-1 text-xs text-warning-fg"
                  >
                    与 {item.conflicts.map((conflict) => conflict.name).join('、')} 共用候选。相同含义请合并；不同含义请修改
                    slug。
                  </p>{/if}{#if item.error_code}<span class="mt-1 block text-xs text-danger-fg"
                    >{taxonomyMessage(item.error_code)}</span
                  >{/if}</td
              ></tr
            >{/each}
        </tbody>
      </table>
    </div>
    <div class="flex flex-wrap gap-3">
      <button
        class={outlineClass}
        type="button"
        disabled={busy || !readyIDs.length || job.status !== 'ready'}
        onclick={() => {
          selected = [...readyIDs];
          confirm = false;
        }}>选择全部成功项</button
      ><button class={outlineClass} type="button" disabled={busy || !dirty} onclick={save}
        >保存候选修改</button
      ><button
        class={primaryClass}
        type="button"
        disabled={busy || dirty || !selected.length || job.status !== 'ready'}
        onclick={() => (confirm = true)}>应用所选（{selected.length}）</button
      >
    </div>
    {#if dirty}<div class="flex flex-wrap items-center gap-3">
        <p class="text-sm text-fg-muted">候选修改尚未保存，请保存或放弃后切换视图。</p>
        <button
          class={outlineClass}
          type="button"
          disabled={busy}
          onclick={() => {
            edits = {};
            confirm = false;
          }}>放弃候选修改</button
        >
      </div>{/if}
    {#if confirm}<section class="grid gap-3 rounded-md bg-subtle p-4" aria-label="确认批量更新">
        <p>确认更新所选 {selected.length} 个标签的 slug？原链接仍可使用。</p>
        <div class="flex gap-3">
          <button
            class={outlineClass}
            type="button"
            disabled={busy}
            onclick={() => (confirm = false)}>返回预览</button
          ><button class={primaryClass} type="button" disabled={busy} onclick={apply}
            >确认更新</button
          >
        </div>
      </section>{/if}
  {/if}
  {#if error}<p class="text-sm text-danger-fg" role="alert">{error}</p>{/if}
</section>

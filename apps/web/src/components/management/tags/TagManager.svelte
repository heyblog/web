<script lang="ts">
  import { tick, untrack } from 'svelte';

  import { readTaxonomy } from '@/api/taxonomy/taxonomy.browser';
  import type { ManagedTag, TaxonomyData } from '@/api/taxonomy/taxonomy.types';
  import { taxonomyMessage } from '@/application/taxonomy/taxonomy.messages';

  import CascadeManager from './CascadeManager.svelte';
  import SlugJobs from './SlugJobs.svelte';
  import TagEditor from './TagEditor.svelte';
  import { inputClass, outlineClass, primaryClass } from './tags.styles';
  import TaxonomyChange from './TaxonomyChange.svelte';

  interface Props {
    initialData: TaxonomyData | null;
    initialError: string | null;
  }
  let { initialData, initialError }: Props = $props();
  let data = $state<TaxonomyData | null>(untrack(() => initialData));
  let error = $state(untrack(() => initialError ?? ''));
  let message = $state('');
  let loading = $state(false);
  let search = $state('');
  let view = $state<'tags' | 'paths' | 'slugs'>('tags');
  let checked = $state<string[]>([]);
  let batchEditing = $state(false);
  let roleFilter = $state('');
  let statusFilter = $state('active');
  let selected = $state<ManagedTag | null>(null);
  let opened = $state(false);
  let panel = $state<'edit' | 'change'>('edit');
  let nextPanel = $state<'edit' | 'change' | 'close' | null>(null);
  let busy = $state(false);
  let dirty = $state(false);
  let dialog: HTMLDialogElement;
  let trigger: HTMLElement | null = null;
  let canonical = $derived(data?.tags ?? []);
  let visible = $derived(
    (data?.tags ?? []).filter(
      (tag) =>
        (!search ||
          `${tag.labels.map((label) => label.name).join(' ')} ${tag.slug}`
            .toLowerCase()
            .includes(search.trim().toLowerCase())) &&
        (!roleFilter || tag.roles.includes(roleFilter)) &&
        (statusFilter === 'all' || (statusFilter === 'active' ? tag.is_enabled : !tag.is_enabled)),
    ),
  );

  $effect(() => {
    if (!opened) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = previous;
    };
  });

  async function refresh(): Promise<void> {
    if (loading || opened) return;
    loading = true;
    error = '';
    const result = await readTaxonomy();
    loading = false;
    if (result.ok) data = result.value;
    else error = taxonomyMessage(result.code);
  }

  async function open(tag: ManagedTag | null, event: MouseEvent): Promise<void> {
    if (loading || opened) return;
    selected = tag;
    trigger = event.currentTarget instanceof HTMLElement ? event.currentTarget : null;
    dirty = false;
    busy = false;
    panel = 'edit';
    nextPanel = null;
    opened = true;
    await tick();
    dialog.showModal();
    await focusStep();
  }

  async function navigate(target: 'edit' | 'change' | 'close'): Promise<void> {
    if (busy) return;
    if (dirty) {
      nextPanel = target;
      await focusStep();
    } else await completeNavigation(target);
  }

  async function focusStep(): Promise<void> {
    await tick();
    dialog.querySelector<HTMLElement>('h2')?.focus();
  }

  async function completeNavigation(target: 'edit' | 'change' | 'close'): Promise<void> {
    nextPanel = null;
    dirty = false;
    if (target === 'close') {
      dialog.close();
      opened = false;
      trigger?.focus();
    } else {
      panel = target;
      await focusStep();
    }
  }

  function saved(value: TaxonomyData): void {
    data = value;
    message = '标签已更新。';
    dirty = false;
    busy = false;
    void completeNavigation('close');
  }
</script>

<div class="grid min-w-0 gap-6">
  <header class="flex flex-wrap items-start justify-between gap-4">
    <div>
      <p class="text-sm text-fg-muted">统一维护站点与文章的分类、标签和历史链接。</p>
      {#if data}<p class="mt-2 text-sm text-fg-muted">
          {canonical.length} 个标签 · {data.cascades.filter((item) => !item.merged_into_id).length} 条分类路径
        </p>{/if}
    </div>
    <div class="flex gap-3">
      <button class={outlineClass} type="button" disabled={loading || opened} onclick={refresh}
        >{loading ? '正在加载…' : '刷新'}</button
      >
      <button
        class={primaryClass}
        type="button"
        disabled={!data || loading || opened}
        onclick={(event) => open(null, event)}>新建标签</button
      >
    </div>
  </header>
  {#if error}<p
      class="rounded-md border border-danger bg-danger-bg p-3 text-sm text-danger-fg"
      role="alert"
    >
      {error}
    </p>{/if}
  {#if message}<p class="text-sm text-success-fg" role="status">{message}</p>{/if}
  {#if data}
    <nav class="flex flex-wrap gap-3" aria-label="标签管理视图">
      <button
        class={outlineClass}
        type="button"
        disabled={batchEditing}
        aria-pressed={view === 'tags'}
        onclick={() => (view = 'tags')}>标签字典</button
      >
      <button
        class={outlineClass}
        type="button"
        disabled={batchEditing}
        aria-pressed={view === 'paths'}
        onclick={() => (view = 'paths')}>分类路径</button
      >
      <button
        class={outlineClass}
        type="button"
        aria-pressed={view === 'slugs'}
        onclick={() => (view = 'slugs')}>批量更新 slug</button
      >
    </nav>
    {#if view === 'paths'}
      <CascadeManager {data} onsaved={(value) => (data = value)} />
    {:else if view === 'slugs'}
      <SlugJobs
        {checked}
        query={search}
        isEnabled={statusFilter === 'all' ? undefined : statusFilter === 'active'}
        onediting={(value) => (batchEditing = value)}
        onapplied={refresh}
      />
    {:else}
      <div class="grid min-w-0 gap-6">
        <section class="grid min-w-0 content-start gap-4" aria-label="标签列表" aria-busy={loading}>
          <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_9rem_9rem]">
            <label class="grid gap-1.5 text-sm"
              >搜索<input
                class={inputClass}
                bind:value={search}
                type="search"
                placeholder="名称或 slug"
              /></label
            >
            <label class="grid gap-1.5 text-sm"
              >使用角色<select class={inputClass} bind:value={roleFilter}
                ><option value="">全部角色</option><option value="PRIMARY">一级分类</option><option
                  value="SECONDARY">二级分类</option
                ><option value="TERTIARY">三级标签</option></select
              ></label
            >
            <label class="grid gap-1.5 text-sm"
              >状态<select class={inputClass} bind:value={statusFilter}
                ><option value="active">已启用</option><option value="disabled">已停用</option
                ><option value="all">全部状态</option></select
              ></label
            >
          </div>
          <p class="text-xs text-fg-muted" role="status">显示 {visible.length} 个标签</p>
          <div class="overflow-x-auto rounded-md border border-line bg-surface">
            <table class="w-full text-left text-sm">
              <caption class="sr-only">标签名称、角色、状态和引用数量</caption>
              <thead class="border-b border-line text-xs text-fg-muted"
                ><tr
                  ><th class="px-4 py-3 font-medium" scope="col">选择</th><th
                    class="px-4 py-3 font-medium"
                    scope="col">标签</th
                  ><th class="px-4 py-3 font-medium whitespace-nowrap" scope="col">分类</th><th
                    class="px-4 py-3 font-medium"
                    scope="col">状态</th
                  ><th class="px-4 py-3 font-medium whitespace-nowrap" scope="col">站点 / 文章</th
                  ><th class="px-4 py-3 font-medium" scope="col">操作</th></tr
                ></thead
              >
              <tbody
                >{#each visible as tag (tag.id)}<tr
                    class="h-12 border-b border-line last:border-0 hover:bg-subtle"
                  >
                    <td class="px-4 py-3"
                      ><input
                        type="checkbox"
                        bind:group={checked}
                        value={tag.id}
                        aria-label={`选择标签 ${tag.name}`}
                      /></td
                    >
                    <th class="min-w-40 px-4 py-3 font-medium" scope="row"
                      ><span class="block wrap-anywhere">{tag.name}</span><span
                        class="mt-1 block text-xs font-normal wrap-anywhere text-fg-muted"
                        >{tag.slug}</span
                      ></th
                    >
                    <td class="px-4 py-3 whitespace-nowrap"
                      >{tag.roles
                        .map(
                          (role) =>
                            ({
                              PRIMARY: '一级',
                              SECONDARY: '二级',
                              TERTIARY: '三级',
                              WARNING: '警告',
                            })[role] ?? role,
                        )
                        .join(' / ') || '未使用'}</td
                    >
                    <td class="px-4 py-3 whitespace-nowrap"
                      ><span
                        class="rounded-full px-2 py-1 text-xs"
                        class:bg-success-bg={tag.is_enabled}
                        class:text-success-fg={tag.is_enabled}
                        class:bg-subtle={!tag.is_enabled}
                        >{tag.is_enabled ? '已启用' : '已停用'}</span
                      ></td
                    >
                    <td class="px-4 py-3 whitespace-nowrap tabular-nums"
                      >{tag.site_count} / {tag.article_count}</td
                    >
                    <td class="px-4 py-3"
                      ><button
                        class="min-h-11 rounded-md px-3 text-sm text-tint-fg hover:bg-subtle"
                        type="button"
                        aria-label={`管理标签 ${tag.name}`}
                        disabled={loading || opened}
                        onclick={(event) => open(tag, event)}>管理</button
                      ></td
                    >
                  </tr>{/each}</tbody
              >
            </table>
            {#if !visible.length}<p class="p-8 text-center text-sm text-fg-muted">
                没有符合条件的标签，请调整筛选。
              </p>{/if}
          </div>
        </section>
      </div>
    {/if}
  {/if}
</div>

<dialog
  bind:this={dialog}
  aria-labelledby="tag-dialog-title"
  aria-describedby="tag-dialog-description"
  class={selected
    ? 'fixed inset-y-0 right-0 left-auto m-0 h-dvh max-h-dvh w-full max-w-full border-l border-line bg-surface p-0 text-fg shadow-md backdrop:bg-overlay sm:w-120'
    : 'm-auto max-h-[calc(100dvh-2rem)] w-[min(calc(100%-2rem),30rem)] rounded-md border border-line bg-surface p-0 text-fg shadow-md backdrop:bg-overlay'}
  oncancel={(event) => {
    event.preventDefault();
    void navigate('close');
  }}
  onclick={(event) => {
    if (event.target === dialog) {
      const rect = dialog.getBoundingClientRect();
      if (
        event.clientX < rect.left ||
        event.clientX > rect.right ||
        event.clientY < rect.top ||
        event.clientY > rect.bottom
      )
        void navigate('close');
    }
  }}
>
  {#if opened && data}
    <div class="flex max-h-[inherit] min-h-0 flex-col" class:h-full={!!selected}>
      <header class="flex shrink-0 items-start justify-between gap-3 border-b border-line p-6">
        <div class="min-w-0">
          <h2 id="tag-dialog-title" class="text-lg font-bold wrap-anywhere" tabindex="-1">
            {nextPanel ? '放弃未保存的内容？' : selected ? selected.name : '新建标签'}
          </h2>
          <p id="tag-dialog-description" class="mt-1 text-sm text-fg-muted">
            {nextPanel ? '当前填写内容尚未保存。' : panel === 'change' ? '合并标签' : '标签信息'}
          </p>
        </div>
        <button class={outlineClass} type="button" disabled={busy} onclick={() => navigate('close')}
          >关闭</button
        >
      </header>
      <div class="min-h-0 overflow-y-auto p-6">
        {#if nextPanel}
          <div class="flex flex-wrap gap-3">
            <button
              class={outlineClass}
              type="button"
              onclick={async () => {
                nextPanel = null;
                await focusStep();
              }}>继续编辑</button
            ><button
              class={primaryClass}
              type="button"
              onclick={() => nextPanel && completeNavigation(nextPanel)}>放弃内容</button
            >
          </div>
        {/if}
        <div hidden={nextPanel !== null}>
          {#if selected}<div class="mb-5 flex gap-3">
              <button
                class={outlineClass}
                type="button"
                aria-pressed={panel === 'edit'}
                disabled={busy || panel === 'edit'}
                onclick={() => navigate('edit')}>编辑信息</button
              ><button
                class={outlineClass}
                type="button"
                aria-pressed={panel === 'change'}
                disabled={busy || panel === 'change'}
                onclick={() => navigate('change')}>合并标签</button
              >
            </div>{/if}
          {#key `${selected?.id ?? 'new'}-${panel}`}
            {#if panel === 'change' && selected}<TaxonomyChange
                tag={selected}
                {data}
                onsaved={saved}
                onbusy={(value) => (busy = value)}
                ondirty={(value) => (dirty = value)}
              />
            {:else}<TagEditor
                tag={selected}
                {data}
                onsaved={saved}
                onbusy={(value) => (busy = value)}
                ondirty={(value) => (dirty = value)}
              />{/if}
          {/key}
        </div>
      </div>
    </div>
  {/if}
</dialog>

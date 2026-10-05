<script lang="ts">
  import { applyChange, createCascade, previewChange } from '@/api/taxonomy/taxonomy.browser';
  import type { ChangePreview, TaxonomyChange, TaxonomyData } from '@/api/taxonomy/taxonomy.types';
  import { taxonomyBlocker, taxonomyMessage } from '@/application/taxonomy/taxonomy.messages';

  import { inputClass, outlineClass, primaryClass } from './tags.styles';

  let { data, onsaved }: { data: TaxonomyData; onsaved: (data: TaxonomyData) => void } = $props();
  let scope = $state<'SITE' | 'ARTICLE'>('SITE');
  let source = $state('');
  let mode = $state<'create' | 'path_update' | 'path_merge'>('create');
  let primary = $state('');
  let secondary = $state('');
  let target = $state('');
  let key = $state('');
  let enabled = $state(true);
  let busy = $state(false);
  let error = $state('');
  let preview = $state<ChangePreview | null>(null);
  let inspected = $state<TaxonomyChange | null>(null);
  const paths = $derived(
    data.cascades.filter((path) => path.scope === scope && !path.merged_into_id),
  );
  const label = (id: string) => data.tags.find((tag) => tag.id === id)?.name ?? id;
  function resetPreview(): void {
    preview = null;
    inspected = null;
    error = '';
  }
  function edit(id: string): void {
    const path = paths.find((item) => item.id === id);
    if (!path) return;
    source = path.id;
    mode = 'path_update';
    primary = path.level1_tag_id;
    secondary = path.level2_tag_id;
    enabled = path.is_enabled;
    target = '';
    resetPreview();
  }
  async function submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (busy) return;
    busy = true;
    error = '';
    if (mode === 'create') {
      const result = await createCascade({
        scope,
        primary_id: primary,
        secondary_id: secondary,
        taxonomy_key: key.trim(),
        expected_revision: data.revision,
      });
      if (result.ok) {
        onsaved(result.value);
        key = '';
      } else error = taxonomyMessage(result.code);
    } else {
      const input: TaxonomyChange =
        mode === 'path_merge'
          ? { kind: mode, source_id: source, target_id: target, expected_revision: data.revision }
          : {
              kind: mode,
              source_id: source,
              primary_id: primary,
              secondary_id: secondary,
              is_enabled: enabled,
              expected_revision: data.revision,
            };
      const result = await previewChange(input);
      if (result.ok) {
        preview = result.value;
        inspected = input;
      } else error = taxonomyMessage(result.code);
    }
    busy = false;
  }
  async function apply(): Promise<void> {
    if (busy || !preview || !inspected || preview.blockers.length) return;
    busy = true;
    const result = await applyChange(inspected, preview.fingerprint);
    busy = false;
    resetPreview();
    if (result.ok) onsaved(result.value);
    else error = taxonomyMessage(result.code);
  }
</script>

<section class="grid gap-5" aria-label="分类路径" aria-busy={busy}>
  <label class="grid gap-1.5 text-sm"
    >使用范围<select
      class={inputClass}
      bind:value={scope}
      disabled={busy}
      onchange={() => {
        mode = 'create';
        source = '';
        resetPreview();
      }}><option value="SITE">站点</option><option value="ARTICLE">文章</option></select
    ></label
  >
  <div class="overflow-x-auto rounded-md border border-line">
    <table class="w-full text-left text-sm">
      <caption class="sr-only">分类路径与状态</caption><thead
        ><tr
          ><th class="p-3" scope="col">分类路径</th><th class="p-3" scope="col">状态</th><th
            class="p-3"
            scope="col">操作</th
          ></tr
        ></thead
      ><tbody>
        {#each paths as path (path.id)}<tr class="border-t border-line"
            ><th class="p-3 font-medium" scope="row"
              >{label(path.level1_tag_id)} → {label(path.level2_tag_id)}</th
            ><td class="p-3">{path.is_enabled ? '已启用' : '已停用'}</td><td class="p-3"
              ><button
                class={outlineClass}
                type="button"
                disabled={busy}
                onclick={() => edit(path.id)}>编辑路径</button
              ></td
            ></tr
          >{/each}
      </tbody>
    </table>
  </div>
  <form
    class="grid gap-5 rounded-md border border-line bg-surface p-5"
    onsubmit={submit}
    onchange={resetPreview}
  >
    <h2 class="text-base font-semibold">{mode === 'create' ? '新建路径' : '修改路径'}</h2>
    <div class="flex flex-wrap gap-3">
      <button
        class={outlineClass}
        type="button"
        disabled={busy}
        onclick={() => {
          mode = 'create';
          source = '';
          resetPreview();
        }}>新建路径</button
      >{#if source}<button
          class={outlineClass}
          type="button"
          disabled={busy}
          aria-pressed={mode === 'path_update'}
          onclick={() => {
            mode = 'path_update';
            resetPreview();
          }}>编辑分类与状态</button
        ><button
          class={outlineClass}
          type="button"
          disabled={busy}
          aria-pressed={mode === 'path_merge'}
          onclick={() => {
            mode = 'path_merge';
            resetPreview();
          }}>合并路径</button
        >{/if}
    </div>
    {#if mode === 'path_merge'}
      <label class="grid gap-1.5 text-sm"
        >保留的路径<select class={inputClass} bind:value={target} required disabled={busy}
          ><option value="">请选择</option
          >{#each paths.filter((path) => path.id !== source && path.is_enabled) as path (path.id)}<option
              value={path.id}>{label(path.level1_tag_id)} → {label(path.level2_tag_id)}</option
            >{/each}</select
        ></label
      >
    {:else}
      <div class="grid gap-4 sm:grid-cols-2">
        <label class="grid gap-1.5 text-sm"
          >一级分类<select class={inputClass} bind:value={primary} required disabled={busy}
            ><option value="">请选择</option
            >{#each data.tags.filter((tag) => tag.is_enabled || tag.id === primary) as tag (tag.id)}<option
                value={tag.id}>{tag.name}</option
              >{/each}</select
          ></label
        >
        <label class="grid gap-1.5 text-sm"
          >二级分类<select class={inputClass} bind:value={secondary} required disabled={busy}
            ><option value="">请选择</option
            >{#each data.tags.filter((tag) => tag.is_enabled || tag.id === secondary) as tag (tag.id)}<option
                value={tag.id}>{tag.name}</option
              >{/each}</select
          ></label
        >
      </div>
      {#if mode === 'create'}<label class="grid gap-1.5 text-sm"
          >路径标识<input
            class={inputClass}
            bind:value={key}
            maxlength="160"
            required
            disabled={busy}
          /></label
        >{:else}<label class="flex min-h-11 items-center gap-3 text-sm"
          ><input type="checkbox" bind:checked={enabled} disabled={busy} />启用路径</label
        >{/if}
    {/if}
    <button class={primaryClass} type="submit" disabled={busy}
      >{busy ? '正在处理…' : mode === 'create' ? '创建路径' : '预览影响'}</button
    >
  </form>
  {#if preview}<section class="grid gap-3 rounded-md bg-subtle p-4" aria-label="路径变更预览">
      <p role="status">
        影响 {preview.site_count} 个站点、{preview.article_count} 篇文章，移除 {preview.removed_duplicates}
        个重复引用。
      </p>
      {#each preview.blockers as blocker (blocker)}<p class="text-sm text-warning-fg">
          {taxonomyBlocker(blocker)}
        </p>{/each}<button
        class={primaryClass}
        type="button"
        disabled={busy || preview.blockers.length > 0}
        onclick={apply}>确认应用</button
      >
    </section>{/if}
  {#if error}<p class="text-sm text-danger-fg" role="alert">{error}</p>{/if}
</section>

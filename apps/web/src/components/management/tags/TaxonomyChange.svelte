<script lang="ts">
  import { tick } from 'svelte';

  import { applyChange, previewChange } from '@/api/taxonomy/taxonomy.browser';
  import type {
    ChangePreview,
    ManagedTag,
    TaxonomyChange,
    TaxonomyData,
  } from '@/api/taxonomy/taxonomy.types';
  import { taxonomyBlocker, taxonomyMessage } from '@/application/taxonomy/taxonomy.messages';

  import { inputClass, outlineClass, primaryClass } from './tags.styles';

  interface Props {
    tag: ManagedTag;
    data: TaxonomyData;
    onsaved: (data: TaxonomyData) => void;
    onbusy: (busy: boolean) => void;
    ondirty: (dirty: boolean) => void;
  }
  let { tag, data, onsaved, onbusy, ondirty }: Props = $props();
  let targetID = $state('');
  let preview = $state<ChangePreview | null>(null);
  let previewInput = $state<TaxonomyChange | null>(null);
  let pending = $state(false);
  let error = $state('');
  let confirmApply = $state(false);
  let section: HTMLElement;
  let mergeTargets = $derived(data.tags.filter((item) => item.is_enabled && item.id !== tag.id));
  $effect(() => onbusy(pending));
  $effect(() => ondirty(!!targetID || preview !== null));

  async function confirmation(show: boolean): Promise<void> {
    confirmApply = show;
    await tick();
    section.querySelector<HTMLElement>(show ? 'h3' : 'select')?.focus();
  }

  function invalidate(): void {
    preview = null;
    previewInput = null;
    confirmApply = false;
    error = '';
  }

  async function inspect(): Promise<void> {
    if (pending) return;
    pending = true;
    error = '';
    const body: TaxonomyChange = {
      kind: 'merge',
      source_id: tag.id,
      target_id: targetID,
      expected_revision: data.revision,
    };
    const result = await previewChange(body);
    pending = false;
    if (result.ok) {
      preview = result.value;
      previewInput = body;
    } else error = taxonomyMessage(result.code);
  }

  async function apply(): Promise<void> {
    if (!preview || !previewInput || pending || preview.blockers.length) return;
    pending = true;
    error = '';
    const result = await applyChange(previewInput, preview.fingerprint);
    pending = false;
    if (result.ok) onsaved(result.value);
    else {
      invalidate();
      error = taxonomyMessage(result.code);
    }
  }
</script>

<section bind:this={section} class="grid gap-5" aria-busy={pending}>
  <p class="text-sm text-fg-muted">合并后，原标签的引用将指向保留的标签。</p>
  {#if confirmApply && preview}
    <h3 class="text-base font-semibold" tabindex="-1">确认应用这次迁移？</h3>
    <p class="text-sm">
      将影响 {preview.site_count} 个站点、{preview.article_count} 篇文章，移除 {preview.removed_duplicates}
      个重复引用。
    </p>
    <div class="flex gap-3">
      <button
        class={outlineClass}
        type="button"
        disabled={pending}
        onclick={() => confirmation(false)}>返回预览</button
      >
      <button class={primaryClass} type="button" disabled={pending} onclick={apply}
        >{pending ? '正在迁移…' : '确认迁移'}</button
      >
    </div>
  {:else}
    <label class="grid gap-1.5 text-sm font-medium"
      >保留的标签<select
        class={inputClass}
        bind:value={targetID}
        disabled={pending}
        onchange={() => invalidate()}
      >
        <option value="">请选择</option>{#each mergeTargets as item (item.id)}<option
            value={item.id}>{item.name}</option
          >{/each}
      </select></label
    >
    <button
      class={preview && !preview.blockers.length ? outlineClass : primaryClass}
      type="button"
      disabled={pending}
      onclick={inspect}>{pending ? '正在预览…' : '预览迁移影响'}</button
    >
    {#if preview}
      <div class="grid gap-3 rounded-md border border-line bg-subtle p-4" role="status">
        <h3 class="text-sm font-semibold">迁移影响</h3>
        {#if !preview.blockers.length}<p class="text-sm">
            {preview.site_count} 个站点 · {preview.article_count} 篇文章 · {preview.removed_duplicates}
            个重复引用
          </p>{/if}
        {#if preview.paths.length}<ul class="grid gap-1 text-xs text-fg-muted">
            {#each preview.paths as path (path.cascade_id)}<li>
                {path.scope === 'SITE' ? '站点' : '文章'}：{path.label}
              </li>{/each}
          </ul>{/if}
        {#if preview.blockers.length}
          <p class="text-sm">完善迁移目标后，请重新预览完整影响。</p>
          <ul class="grid list-disc gap-2 pl-5 text-sm text-warning-fg">
            {#each preview.blockers as blocker (blocker)}<li>{taxonomyBlocker(blocker)}</li>{/each}
          </ul>
        {:else}
          <button
            class={primaryClass}
            type="button"
            disabled={pending}
            onclick={() => confirmation(true)}>应用迁移</button
          >
        {/if}
      </div>
    {/if}
  {/if}
  {#if error}<p class="rounded-md bg-danger-bg p-3 text-sm text-danger-fg" role="alert">
      {error}
    </p>{/if}
</section>

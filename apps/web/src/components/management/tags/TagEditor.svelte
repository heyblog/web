<script lang="ts">
  import { tick, untrack } from 'svelte';

  import { deleteTag, saveTag } from '@/api/taxonomy/taxonomy.browser';
  import type { ManagedTag, TaxonomyData } from '@/api/taxonomy/taxonomy.types';
  import { taxonomyMessage } from '@/application/taxonomy/taxonomy.messages';

  import SlugField from './SlugField.svelte';
  import TagLabels from './TagLabels.svelte';
  import { inputClass, outlineClass, primaryClass } from './tags.styles';

  interface Props {
    tag: ManagedTag | null;
    data: TaxonomyData;
    onsaved: (data: TaxonomyData) => void;
    onbusy: (busy: boolean) => void;
    ondirty: (dirty: boolean) => void;
  }
  let { tag, data, onsaved, onbusy, ondirty }: Props = $props();
  let name = $state(untrack(() => tag?.name ?? ''));
  let slug = $state(untrack(() => tag?.slug ?? ''));
  let description = $state(untrack(() => tag?.description ?? ''));
  let enabled = $state(untrack(() => tag?.is_enabled ?? true));
  let pending = $state(false);
  let labelsBusy = $state(false);
  let labelsDirty = $state(false);
  let generating = $state(false);
  let error = $state('');
  let confirmDelete = $state(false);
  let form: HTMLFormElement;
  let dirty = $derived(
    name !== (tag?.name ?? '') ||
      slug !== (tag?.slug ?? '') ||
      description !== (tag?.description ?? '') ||
      enabled !== (tag?.is_enabled ?? true),
  );
  $effect(() => ondirty(dirty || labelsDirty));
  $effect(() => onbusy(pending || generating || labelsBusy));

  async function deleteStep(confirm: boolean): Promise<void> {
    confirmDelete = confirm;
    await tick();
    form.querySelector<HTMLElement>(confirm ? 'h3' : 'input')?.focus();
  }

  async function save(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (pending || generating || labelsBusy || labelsDirty) return;
    pending = true;
    error = '';
    const common = {
      name: name.trim(),
      slug: slug.trim(),
      description: description.trim(),
      expected_revision: data.revision,
    };
    const result = await saveTag(tag?.id ?? '', tag ? { ...common, is_enabled: enabled } : common);
    pending = false;
    if (result.ok) onsaved(result.value);
    else error = taxonomyMessage(result.code);
  }

  async function remove(): Promise<void> {
    if (!tag || pending || generating || labelsBusy || labelsDirty) return;
    pending = true;
    error = '';
    const result = await deleteTag(tag.id, data.revision);
    pending = false;
    if (result.ok) onsaved(result.value);
    else {
      error = taxonomyMessage(result.code);
      confirmDelete = false;
    }
  }
</script>

<form bind:this={form} class="grid gap-5" onsubmit={save} aria-busy={pending || generating}>
  {#if confirmDelete}
    <h3 class="text-base font-semibold" tabindex="-1">确认删除“{name}”？</h3>
    <p class="text-sm">没有引用的分类路径会一并删除。审核历史或别名仍引用时无法删除。</p>
    <div class="flex gap-3">
      <button
        class={outlineClass}
        type="button"
        disabled={pending}
        onclick={() => deleteStep(false)}>取消</button
      >
      <button
        class="min-h-11 rounded-md bg-danger-solid px-4 font-medium text-primary-fg disabled:opacity-50"
        type="button"
        disabled={pending}
        onclick={remove}>确认删除</button
      >
    </div>
  {:else}
    <label class="grid gap-1.5 text-sm font-medium"
      >名称<input
        class={inputClass}
        bind:value={name}
        required
        maxlength="120"
        disabled={pending || generating || labelsBusy || labelsDirty}
      /></label
    >
    <SlugField
      bind:value={slug}
      {name}
      {description}
      tagID={tag?.id ?? ''}
      disabled={pending || labelsBusy || labelsDirty}
      onbusy={(value) => (generating = value)}
    />
    {#if tag}<p class="text-xs text-fg-muted">
        重命名保留原 slug。修改 slug 后，原链接仍可使用。
      </p>{/if}
    <label class="grid gap-1.5 text-sm font-medium"
      >说明（选填）<textarea
        class="min-h-28 rounded-md border border-line-strong bg-surface p-3"
        bind:value={description}
        maxlength="2000"
        disabled={pending || generating || labelsBusy || labelsDirty}></textarea></label
    >
    {#if tag}
      <label class="flex min-h-11 items-center gap-3 text-sm"
        ><input
          type="checkbox"
          bind:checked={enabled}
          disabled={pending || generating || labelsBusy || labelsDirty}
        />启用标签</label
      >
      <p class="text-xs text-fg-muted">停用后无法新增引用，现有引用仍会显示。</p>
    {/if}
    <div class="flex flex-wrap justify-between gap-3 border-t border-line pt-5">
      <button
        class={primaryClass}
        type="submit"
        disabled={pending || generating || labelsBusy || labelsDirty || !dirty}
        >{pending ? '正在保存…' : '保存标签'}</button
      >
      {#if tag}<button
          class="min-h-11 rounded-md px-3 text-sm text-danger-fg hover:bg-danger-bg disabled:opacity-50"
          type="button"
          disabled={pending || generating || labelsBusy || labelsDirty}
          onclick={() => deleteStep(true)}>删除标签</button
        >{/if}
    </div>
  {/if}
  {#if error}<p class="rounded-md bg-danger-bg p-3 text-sm text-danger-fg" role="alert">
      {error}
    </p>{/if}
</form>

{#if tag && !confirmDelete}
  <TagLabels
    {tag}
    {data}
    disabled={pending || generating || dirty}
    {onsaved}
    onbusy={(value) => (labelsBusy = value)}
    ondirty={(value) => (labelsDirty = value)}
  />
{/if}

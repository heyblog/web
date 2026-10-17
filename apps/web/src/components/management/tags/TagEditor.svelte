<script lang="ts">
  import { tick, untrack } from 'svelte';

  import { deleteTag, saveTag } from '@/api/taxonomy/taxonomy.browser';
  import type { ManagedTag, TaxonomyData } from '@/api/taxonomy/taxonomy.types';
  import { formFeedback, showFieldIssue } from '@/application/forms/field-feedback.browser';
  import { taxonomyMessage } from '@/application/taxonomy/taxonomy.messages';

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
  let description = $state(untrack(() => tag?.description ?? ''));
  let enabled = $state(untrack(() => tag?.is_enabled ?? true));
  let pending = $state(false);
  let error = $state('');
  let confirmDelete = $state(false);
  let form: HTMLFormElement;
  let dirty = $derived(
    name !== (tag?.name ?? '') ||
      description !== (tag?.description ?? '') ||
      enabled !== (tag?.is_enabled ?? true),
  );
  $effect(() => ondirty(dirty));
  $effect(() => onbusy(pending));

  async function deleteStep(confirm: boolean): Promise<void> {
    confirmDelete = confirm;
    await tick();
    form.querySelector<HTMLElement>(confirm ? 'h3' : 'input')?.focus();
  }

  async function save(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (pending) return;
    pending = true;
    error = '';
    const common = {
      name: name.trim(),
      description: description.trim(),
      expected_revision: data.revision,
    };
    const result = await saveTag(tag?.id ?? '', tag ? { ...common, is_enabled: enabled } : common);
    pending = false;
    if (result.ok) onsaved(result.value);
    else {
      const message = taxonomyMessage(result.code);
      if (
        !['invalid_tag_name', 'duplicate_tag_name'].includes(result.code) ||
        !showFieldIssue(form, 'name', message)
      )
        error = message;
    }
  }

  async function remove(): Promise<void> {
    if (!tag || pending) return;
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

<form use:formFeedback bind:this={form} class="grid gap-5" onsubmit={save} aria-busy={pending}>
  {#if confirmDelete}
    <h3 class="text-base font-semibold" tabindex="-1">确认删除“{name}”？</h3>
    <p class="text-sm">没有引用的分类路径会一并删除。审核历史仍引用时无法删除。</p>
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
        disabled={pending}
        name="name"
        data-max-characters="120"
        aria-describedby="field-name-help"
      /><span id="field-name-help" class="text-xs/normal font-normal text-fg-muted"
        >最多 120 个字符；去除首尾空格后，不区分大小写唯一。</span
      ></label
    >
    <label class="grid gap-1.5 text-sm font-medium"
      >说明（选填）<textarea
        class="min-h-28 rounded-md border border-line-strong bg-surface p-3"
        bind:value={description}
        disabled={pending}
        name="description"
        data-max-characters="2000"
        aria-describedby="field-description-help"></textarea><span
        id="field-description-help"
        class="text-xs/normal font-normal text-fg-muted">选填，最多 2000 个字符。</span
      ></label
    >
    {#if tag}
      <label class="flex min-h-11 items-center gap-3 text-sm"
        ><input type="checkbox" bind:checked={enabled} disabled={pending} />启用标签</label
      >
      <p class="text-xs text-fg-muted">停用后无法新增引用，现有引用仍会显示。</p>
    {/if}
    <div class="flex flex-wrap justify-between gap-3 border-t border-line pt-5">
      <button class={primaryClass} type="submit" disabled={pending || !dirty}
        >{pending ? '正在保存…' : '保存标签'}</button
      >
      {#if tag}<button
          class="min-h-11 rounded-md px-3 text-sm text-danger-fg hover:bg-danger-bg disabled:opacity-50"
          type="button"
          disabled={pending}
          onclick={() => deleteStep(true)}>删除标签</button
        >{/if}
    </div>
  {/if}
  {#if error}<p class="rounded-md bg-danger-bg p-3 text-sm text-danger-fg" role="alert">
      {error}
    </p>{/if}
</form>

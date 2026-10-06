<script lang="ts">
  import { deleteLabel, saveLabel, setDefaultLabel } from '@/api/taxonomy/taxonomy.browser';
  import type { ManagedTag, TaxonomyData } from '@/api/taxonomy/taxonomy.types';
  import { taxonomyMessage } from '@/application/taxonomy/taxonomy.messages';

  import { inputClass, outlineClass } from './tags.styles';

  interface Props {
    tag: ManagedTag;
    data: TaxonomyData;
    disabled: boolean;
    onsaved: (data: TaxonomyData) => void;
    onbusy: (busy: boolean) => void;
    ondirty: (dirty: boolean) => void;
  }
  let { tag, data, disabled, onsaved, onbusy, ondirty }: Props = $props();
  let editingID = $state('');
  let name = $state('');
  let enabled = $state(true);
  let pending = $state(false);
  let error = $state('');
  let removeID = $state('');
  $effect(() => onbusy(pending));
  $effect(() => ondirty(Boolean(name || editingID || removeID)));

  async function action(kind: 'save' | 'default' | 'delete', id = editingID): Promise<void> {
    if (pending || disabled) return;
    pending = true;
    error = '';
    try {
      const result =
        kind === 'default'
          ? await setDefaultLabel(tag.id, id, data.revision)
          : kind === 'delete'
            ? await deleteLabel(tag.id, id, data.revision)
            : await saveLabel(tag.id, id, {
                name: name.trim(),
                ...(id ? { is_enabled: enabled } : {}),
                expected_revision: data.revision,
              });
      if (result.ok) onsaved(result.value);
      else error = taxonomyMessage(result.code);
    } finally {
      pending = false;
    }
  }
</script>

<section class="grid gap-3 border-t border-line pt-5" aria-label="标签名称" aria-busy={pending}>
  <h3 class="text-sm font-semibold">名称与同义词</h3>
  <ul class="grid gap-3">
    {#each tag.labels as label (label.id)}
      <li class="grid gap-2 rounded-md border border-line p-3">
        <div class="flex flex-wrap items-center gap-2 text-sm">
          <span class="min-w-0 flex-1 wrap-anywhere">{label.name}</span>
          {#if label.id === tag.default_label_id}<span class="text-xs text-tint-fg">默认名称</span
            >{/if}
          {#if !label.is_enabled}<span class="text-xs text-fg-muted">已停用</span>{/if}
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            class={outlineClass}
            type="button"
            disabled={disabled || pending}
            onclick={() => {
              editingID = label.id;
              name = label.name;
              enabled = label.is_enabled;
              removeID = '';
            }}>编辑名称</button
          >
          {#if label.id !== tag.default_label_id && label.is_enabled}
            <button
              class={outlineClass}
              type="button"
              disabled={disabled || pending || Boolean(name)}
              onclick={() => action('default', label.id)}>设为默认</button
            >
          {/if}
          {#if label.id !== tag.default_label_id}
            <button
              class="min-h-11 rounded-md px-3 text-sm text-danger-fg hover:bg-danger-bg sm:min-h-10"
              type="button"
              disabled={disabled || pending || Boolean(name)}
              onclick={() => (removeID = label.id)}>删除名称</button
            >
          {/if}
        </div>
        {#if removeID === label.id}
          <div class="flex flex-wrap items-center gap-2">
            <p class="text-sm">确认删除“{label.name}”？</p>
            <button
              class={outlineClass}
              type="button"
              disabled={pending}
              onclick={() => (removeID = '')}>取消</button
            >
            <button
              class={outlineClass}
              type="button"
              disabled={pending}
              onclick={() => action('delete', label.id)}>确认删除</button
            >
          </div>
        {/if}
      </li>
    {/each}
  </ul>
  <label class="grid gap-1.5 text-sm font-medium"
    >{editingID ? '编辑名称' : '新增同义名称'}
    <input class={inputClass} bind:value={name} maxlength="120" disabled={disabled || pending} />
  </label>
  {#if editingID}
    <label class="flex min-h-11 items-center gap-3 text-sm"
      ><input
        type="checkbox"
        bind:checked={enabled}
        disabled={disabled || pending || editingID === tag.default_label_id}
      />启用名称</label
    >
  {/if}
  <div class="flex flex-wrap gap-2">
    <button
      class={outlineClass}
      type="button"
      disabled={disabled || pending || !name.trim()}
      onclick={() => action('save')}>{editingID ? '保存名称' : '添加名称'}</button
    >
    {#if editingID || name}<button
        class={outlineClass}
        type="button"
        disabled={pending}
        onclick={() => {
          editingID = '';
          name = '';
          error = '';
        }}>取消编辑</button
      >{/if}
  </div>
  {#if error}<p class="text-sm text-danger-fg" role="alert">{error}</p>{/if}
</section>

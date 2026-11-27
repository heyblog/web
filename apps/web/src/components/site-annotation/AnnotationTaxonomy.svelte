<script lang="ts">
  import {
    type AnnotationOptions,
    componentRoles,
    tagRoles,
  } from '@/api/site-annotation/annotation.types';
  import type { AnnotationDraft } from '@/application/site-annotation/annotation-draft';
  import {
    componentRoleLabels,
    tagRoleLabels,
  } from '@/application/site-annotation/annotation-view';

  import {
    buttonClass,
    fieldClass,
    inputClass,
    rowClass,
    sectionClass,
    warningClass,
  } from './annotation.styles';

  let {
    draft = $bindable(),
    options,
    allocateKey,
  }: {
    draft: AnnotationDraft;
    options: AnnotationOptions | null;
    allocateKey: () => number;
  } = $props();

  const tagOptions = $derived(
    options?.tags.filter((tag) => tag.level === 3 || tag.level === undefined) ?? [],
  );
  const canAddTag = $derived(
    tagOptions.some((option) => !draft.tags.some((tag) => tag.id === option.id)) &&
      draft.tags.every((tag) => Boolean(tag.id)),
  );
  const currentCascade = $derived(
    options?.cascades.find((cascade) => cascade.id === draft.tag_cascade_id),
  );

  function addTag() {
    if (canAddTag) {
      draft.tags.push({ key: allocateKey(), id: '', name: '', role: 'TERTIARY', level: 3 });
    }
  }

  function addComponent() {
    if (!options?.components.length) return;
    const role = draft.components.some((component) => component.role === 'SITE_PROGRAM')
      ? 'OTHER'
      : 'SITE_PROGRAM';
    draft.components.push({ key: allocateKey(), id: '', name: '', role });
  }
</script>

<section class={sectionClass} aria-labelledby="annotation-taxonomy">
  <h2 id="annotation-taxonomy" class="text-lg font-semibold">分类与标签</h2>
  {#if !options}<p class={warningClass}>暂时无法加载分类、标签和组件选项。刷新页面可重试。</p>{/if}
  <label class={fieldClass}
    >分类路径
    <select
      class={inputClass}
      bind:value={draft.tag_cascade_id}
      disabled={!options?.cascades.length}
    >
      <option value="" disabled>未设置分类</option>
      {#if draft.tag_cascade_id && !currentCascade}
        <option value={draft.tag_cascade_id}>当前分类（选项不可用）</option>
      {/if}
      {#each options?.cascades ?? [] as cascade (cascade.id)}
        <option value={cascade.id}>{cascade.level1.name} / {cascade.level2.name}</option>
      {/each}
    </select>
  </label>
  <div class="flex flex-wrap items-center justify-between gap-3">
    <h3 class="text-base font-semibold">标签</h3>
    <button
      class={buttonClass}
      type="button"
      onclick={addTag}
      disabled={!canAddTag || draft.tags.length >= 40}>添加标签</button
    >
  </div>
  {#if draft.tags.length === 0}<p class="text-sm text-fg-muted">暂无标签。</p>{/if}
  {#each draft.tags as tag (tag.key)}
    <div class={rowClass}>
      <label class={fieldClass}
        >标签
        <select class={inputClass} bind:value={tag.id} disabled={!options} required>
          <option value="" disabled>选择标签</option>
          {#if tag.id && !tagOptions.some((option) => option.id === tag.id)}
            <option value={tag.id}>{tag.name}（当前选项）</option>
          {/if}
          {#each tagOptions as option (option.id)}
            <option
              value={option.id}
              disabled={draft.tags.some((entry) => entry.key !== tag.key && entry.id === option.id)}
              >{option.name}</option
            >
          {/each}
        </select>
      </label>
      <div class="flex items-end gap-3">
        <label class={`${fieldClass} flex-1`}
          >用途
          <select
            class={inputClass}
            bind:value={tag.role}
            disabled={Boolean(tag.id) && !tagOptions.some((option) => option.id === tag.id)}
          >
            {#each tagRoles as role (role)}<option value={role}>{tagRoleLabels[role]}</option
              >{/each}
          </select>
        </label>
        <button
          class={buttonClass}
          type="button"
          onclick={() => {
            draft.tags = draft.tags.filter((entry) => entry.key !== tag.key);
          }}>移除标签</button
        >
      </div>
    </div>
  {/each}
</section>

<section class={sectionClass} aria-labelledby="annotation-components">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <h2 id="annotation-components" class="text-lg font-semibold">站点组件</h2>
    <button
      class={buttonClass}
      type="button"
      onclick={addComponent}
      disabled={!options?.components.length ||
        draft.components.length >= 50 ||
        draft.components.some((component) => !component.id)}>添加组件</button
    >
  </div>
  {#if draft.components.length === 0}<p class="text-sm text-fg-muted">暂无组件。</p>{/if}
  {#each draft.components as component (component.key)}
    {@const available = options?.components.some((option) => option.id === component.id)}
    <div class={rowClass}>
      <label class={fieldClass}
        >组件
        <select class={inputClass} bind:value={component.id} disabled={!options} required>
          <option value="" disabled>选择组件</option>
          {#if component.id && !available}<option value={component.id}
              >{component.name}（当前选项）</option
            >{/if}
          {#each options?.components ?? [] as option (option.id)}<option value={option.id}
              >{option.name}</option
            >{/each}
        </select>
      </label>
      <div class="flex items-end gap-3">
        <label class={`${fieldClass} flex-1`}
          >用途
          <select
            class={inputClass}
            bind:value={component.role}
            disabled={Boolean(component.id) && !available}
          >
            {#each componentRoles as role (role)}
              <option
                value={role}
                disabled={role === 'SITE_PROGRAM' &&
                  draft.components.some(
                    (entry) => entry.key !== component.key && entry.role === role,
                  )}>{componentRoleLabels[role]}</option
              >
            {/each}
          </select>
        </label>
        <button
          class={buttonClass}
          type="button"
          onclick={() => {
            draft.components = draft.components.filter((entry) => entry.key !== component.key);
          }}>移除组件</button
        >
      </div>
    </div>
  {/each}
</section>

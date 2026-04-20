<script lang="ts">
  import type {
    SiteSnapshotDraft,
    SiteSubmissionOptions,
  } from '@/application/management/site-management.snapshot';
  import {
    WORKSPACE_INPUT_CLASS,
    WORKSPACE_SELECT_CHEVRON_STYLE,
    WORKSPACE_SELECT_CLASS,
    WORKSPACE_TEXTAREA_CLASS,
  } from '@/components/site-submission/site-submission-workspace.constants';
  import TagMultiCombobox from '@/shared/ui/TagMultiCombobox.svelte';

  let {
    draft = $bindable(),
    options,
    disabled = false,
    idPrefix = 'site-management-fields',
    fieldAlerts = {},
  }: {
    draft: SiteSnapshotDraft;
    options: SiteSubmissionOptions;
    disabled?: boolean;
    idPrefix?: string;
    fieldAlerts?: Partial<Record<string, { label: string; value: string }>>;
  } = $props();

  type DraftTextField = 'name' | 'url' | 'sign' | 'main_tag_id';

  const syncDraft = (): void => {
    draft = { ...draft };
  };

  const updateDraftTextField = (field: DraftTextField, value: string): void => {
    draft[field] = value;
    syncDraft();
  };
</script>

<div class="space-y-4">
  <p class="text-xs tracking-[0.16em] text-(--color-fg-3)">博客信息</p>
  <div class="grid gap-4 md:grid-cols-2">
    <div class="space-y-2 md:col-span-2">
      {#if fieldAlerts.name}
        <div
          class="rounded-sm border border-[color-mix(in_srgb,var(--color-fail)_32%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-fail)_8%,transparent)] px-3 py-2"
        >
          <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改前</p>
          <p class="mt-1 text-xs whitespace-pre-wrap text-(--color-fg)">{fieldAlerts.name.value}</p>
        </div>
      {/if}
      <label class="block text-sm" for={`${idPrefix}-name`}
        >站点名称<span class="ml-1 text-(--color-fail)" aria-hidden="true">✱</span></label
      >
      <input
        id={`${idPrefix}-name`}
        class={WORKSPACE_INPUT_CLASS}
        {disabled}
        value={draft.name}
        oninput={(event) =>
          updateDraftTextField('name', (event.currentTarget as HTMLInputElement).value)}
        placeholder="例如：Example Blog"
      />
    </div>

    <div class="space-y-2 md:col-span-2">
      {#if fieldAlerts.url}
        <div
          class="rounded-sm border border-[color-mix(in_srgb,var(--color-fail)_32%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-fail)_8%,transparent)] px-3 py-2"
        >
          <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改前</p>
          <p class="mt-1 text-xs whitespace-pre-wrap text-(--color-fg)">{fieldAlerts.url.value}</p>
        </div>
      {/if}
      <label class="block text-sm" for={`${idPrefix}-url`}
        >站点地址<span class="ml-1 text-(--color-fail)" aria-hidden="true">✱</span></label
      >
      <input
        id={`${idPrefix}-url`}
        class={WORKSPACE_INPUT_CLASS}
        {disabled}
        value={draft.url}
        oninput={(event) =>
          updateDraftTextField('url', (event.currentTarget as HTMLInputElement).value)}
        placeholder="https://example.com"
      />
    </div>

    <div class="space-y-2 md:col-span-2">
      {#if fieldAlerts.sign}
        <div
          class="rounded-sm border border-[color-mix(in_srgb,var(--color-fail)_32%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-fail)_8%,transparent)] px-3 py-2"
        >
          <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改前</p>
          <p class="mt-1 text-xs whitespace-pre-wrap text-(--color-fg)">{fieldAlerts.sign.value}</p>
        </div>
      {/if}
      <label class="block text-sm" for={`${idPrefix}-sign`}
        >站点简介<span class="ml-1 text-(--color-fail)" aria-hidden="true">✱</span></label
      >
      <textarea
        id={`${idPrefix}-sign`}
        class={WORKSPACE_TEXTAREA_CLASS}
        {disabled}
        oninput={(event) =>
          updateDraftTextField('sign', (event.currentTarget as HTMLTextAreaElement).value)}
        placeholder="请简要介绍站点内容、或者是个性签名。">{draft.sign}</textarea
      >
    </div>
  </div>

  <div class="grid gap-4 md:grid-cols-2">
    <div class="space-y-2">
      {#if fieldAlerts.main_tag}
        <div
          class="rounded-sm border border-[color-mix(in_srgb,var(--color-fail)_32%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-fail)_8%,transparent)] px-3 py-2"
        >
          <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改前</p>
          <p class="mt-1 text-xs whitespace-pre-wrap text-(--color-fg)">
            {fieldAlerts.main_tag.value}
          </p>
        </div>
      {/if}
      <label class="block text-sm" for={`${idPrefix}-main-tag`}
        >主分类<span class="ml-1 text-(--color-fail)" aria-hidden="true">✱</span></label
      >
      <select
        id={`${idPrefix}-main-tag`}
        class={WORKSPACE_SELECT_CLASS}
        style={WORKSPACE_SELECT_CHEVRON_STYLE}
        {disabled}
        value={draft.main_tag_id}
        onchange={(event) =>
          updateDraftTextField('main_tag_id', (event.currentTarget as HTMLSelectElement).value)}
      >
        <option value="">请选择主分类</option>
        {#each options.main_tags as item (item.id)}
          <option value={item.id}>{item.name}</option>
        {/each}
      </select>
    </div>

    <div class="space-y-2">
      {#if fieldAlerts.sub_tags}
        <div
          class="rounded-sm border border-[color-mix(in_srgb,var(--color-fail)_32%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-fail)_8%,transparent)] px-3 py-2"
        >
          <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改前</p>
          <p class="mt-1 text-xs whitespace-pre-wrap text-(--color-fg)">
            {fieldAlerts.sub_tags.value}
          </p>
        </div>
      {/if}
      <label class="block text-sm" for={`${idPrefix}-sub-tags-combobox`}>子分类</label>
      <TagMultiCombobox
        inputId={`${idPrefix}-sub-tags-combobox`}
        options={options.sub_tags}
        bind:items={draft.sub_tags}
        {disabled}
      />
    </div>
  </div>
</div>

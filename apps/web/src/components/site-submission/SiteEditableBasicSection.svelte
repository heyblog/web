<script lang="ts">
  import type { AutoFillFieldKey } from '@/application/site-submission/site-submission.browser-workspace';
  import type {
    FieldErrors,
    SiteSubmissionOptionsResult,
  } from '@/application/site-submission/site-submission.service';
  import TagMultiCombobox from '@/shared/ui/TagMultiCombobox.svelte';

  import type { CommonSiteForm } from './site-editable-fields.types';

  let {
    form = $bindable(),
    errors = {},
    options,
    optionsPending = false,
    disabled = false,
    idPrefix = 'site-fields',
    inputClass = '',
    textAreaClass = '',
    selectClass = '',
    selectChevronStyle = '',
    withInputStateClass = (base) => base,
    isAutoFillMissing = () => false,
    clearAutoFillMissing = () => {},
    updateUrl = undefined,
    applyAddressInference = undefined,
    runAutoFill = undefined,
  }: {
    form: CommonSiteForm;
    errors?: FieldErrors;
    options: SiteSubmissionOptionsResult;
    optionsPending?: boolean;
    disabled?: boolean;
    idPrefix?: string;
    inputClass?: string;
    textAreaClass?: string;
    selectClass?: string;
    selectChevronStyle?: string;
    withInputStateClass?: (base: string, warned: boolean, missing: boolean) => string;
    isAutoFillMissing?: (field: AutoFillFieldKey) => boolean;
    clearAutoFillMissing?: (field: AutoFillFieldKey) => void;
    updateUrl?: ((value: string) => void) | undefined;
    applyAddressInference?: (() => void) | undefined;
    runAutoFill?: (() => Promise<void> | void) | undefined;
  } = $props();

  const syncForm = (): void => {
    form = { ...form };
  };

  const handleUrlInput = (value: string): void => {
    if (updateUrl) {
      updateUrl(value);
      return;
    }

    form.url = value;
    syncForm();
  };

  let showAutofillActions = $derived(Boolean(applyAddressInference || runAutoFill));
</script>

<div class="space-y-4">
  <p class="text-xs tracking-[0.16em] text-(--color-fg-3)">博客信息</p>
  <div class="grid gap-4 md:grid-cols-2">
    <div class="space-y-2 md:col-span-2">
      <label class="block text-sm" for={`${idPrefix}-name`}
        >站点名称<span class="ml-1 text-(--color-fail)" aria-hidden="true">✱</span></label
      >
      <input
        id={`${idPrefix}-name`}
        class={withInputStateClass(inputClass, false, isAutoFillMissing('name'))}
        {disabled}
        value={form.name}
        oninput={(event) => {
          form.name = (event.currentTarget as HTMLInputElement).value;
          clearAutoFillMissing('name');
          syncForm();
        }}
        placeholder="例如：Example Blog"
      />
      {#if errors.name}<p class="text-xs text-(--color-fail)">{errors.name}</p>{/if}
    </div>

    <div class="space-y-2 md:col-span-2">
      <label class="block text-sm" for={`${idPrefix}-url`}
        >站点地址<span class="ml-1 text-(--color-fail)" aria-hidden="true">✱</span></label
      >
      <div class="flex items-stretch gap-3">
        <input
          id={`${idPrefix}-url`}
          class={`${inputClass} flex-1`}
          {disabled}
          value={form.url}
          oninput={(event) => handleUrlInput((event.currentTarget as HTMLInputElement).value)}
          placeholder="https://example.com"
        />
        {#if showAutofillActions}
          <button
            class="inline-flex min-h-11 shrink-0 items-center justify-center rounded-md border border-(--color-line-med) px-4 text-sm"
            type="button"
            onclick={() => applyAddressInference?.()}
            {disabled}
          >
            同步基础地址
          </button>
          <button
            class="inline-flex min-h-11 shrink-0 items-center justify-center rounded-md border border-red-700/20 px-4 text-sm font-medium text-red-700 dark:border-red-400/20 dark:text-red-400"
            type="button"
            onclick={() => void runAutoFill?.()}
            {disabled}
          >
            自动填写
          </button>
        {/if}
      </div>
      {#if errors.url}<p class="text-xs text-(--color-fail)">{errors.url}</p>{/if}
    </div>

    <div class="space-y-2 md:col-span-2">
      <label class="block text-sm" for={`${idPrefix}-sign`}
        >站点简介<span class="ml-1 text-(--color-fail)" aria-hidden="true">✱</span></label
      >
      <textarea
        id={`${idPrefix}-sign`}
        class={withInputStateClass(textAreaClass, false, isAutoFillMissing('sign'))}
        {disabled}
        oninput={(event) => {
          form.sign = (event.currentTarget as HTMLTextAreaElement).value;
          clearAutoFillMissing('sign');
          syncForm();
        }}
        placeholder="请简要介绍站点内容、或者是个性签名。">{form.sign}</textarea
      >
      {#if errors.sign}<p class="text-xs text-(--color-fail)">{errors.sign}</p>{/if}
    </div>
  </div>

  <div class="grid gap-4 md:grid-cols-2">
    <div class="space-y-2">
      <label class="block text-sm" for={`${idPrefix}-main-tag`}
        >主分类<span class="ml-1 text-(--color-fail)" aria-hidden="true">✱</span></label
      >
      <select
        id={`${idPrefix}-main-tag`}
        class={selectClass}
        style={selectChevronStyle}
        disabled={disabled || optionsPending}
        value={form.main_tag_id}
        onchange={(event) => {
          form.main_tag_id = (event.currentTarget as HTMLSelectElement).value;
          syncForm();
        }}
      >
        <option value="">请选择主分类</option>
        {#each options.main_tags as item (item.id)}
          <option value={item.id}>{item.name}</option>
        {/each}
      </select>
      {#if errors.main_tag_id}<p class="text-xs text-(--color-fail)">{errors.main_tag_id}</p>{/if}
    </div>

    <div class="space-y-2">
      <label class="block text-sm" for={`${idPrefix}-sub-tags-combobox`}>子分类</label>
      <TagMultiCombobox
        inputId={`${idPrefix}-sub-tags-combobox`}
        options={options.sub_tags}
        bind:items={form.sub_tags}
        disabled={disabled || optionsPending}
      />
    </div>
  </div>
</div>

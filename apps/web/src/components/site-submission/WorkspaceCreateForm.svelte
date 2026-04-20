<script lang="ts">
  import { type CustomProgramDraft } from '@/application/site-submission/site-submission.browser-form';
  import type { AutoFillFieldKey } from '@/application/site-submission/site-submission.browser-workspace';
  import {
    type CreateSubmissionFormState,
    FEED_TYPE_OPTIONS,
    type FieldErrors,
    type SiteSubmissionOptionsResult,
  } from '@/application/site-submission/site-submission.service';

  import SiteEditableFields from './SiteEditableFields.svelte';

  let {
    autoFillPending = false,
    autoFillTarget = null,
    submitCreate,
    createForm,
    createErrors = {},
    createPending = false,
    inputClass = '',
    textAreaClass = '',
    selectClass = '',
    selectChevronStyle = '',
    withInputStateClass,
    isAutoFillMissing,
    clearAutoFillMissing,
    fieldNeedsRefinement,
    updateCreateUrl,
    applyAddressInference,
    runAutoFill,
    addFeed,
    removeFeed,
    updateFeedName,
    updateFeedType,
    updateFeedUrl,
    selectDefaultFeed,
    optionsPending = false,
    options,
    createProgramSelectedId = '',
    selectProgramForCreate,
    applyProgramCustomDraftForCreate,
    trimText,
  }: {
    autoFillPending?: boolean;
    autoFillTarget?: 'create' | 'update' | null;
    submitCreate: () => Promise<void>;
    createForm: CreateSubmissionFormState;
    createErrors?: FieldErrors;
    createPending?: boolean;
    inputClass?: string;
    textAreaClass?: string;
    selectClass?: string;
    selectChevronStyle?: string;
    withInputStateClass: (base: string, warned: boolean, missing: boolean) => string;
    isAutoFillMissing: (kind: 'create' | 'update', field: AutoFillFieldKey) => boolean;
    clearAutoFillMissing: (kind: 'create' | 'update', field: AutoFillFieldKey) => void;
    fieldNeedsRefinement: (kind: 'create' | 'update', value: string) => boolean;
    updateCreateUrl: (value: string) => void;
    applyAddressInference: (kind: 'create' | 'update') => void;
    runAutoFill: (kind: 'create' | 'update') => Promise<void>;
    addFeed: (kind: 'create' | 'update') => void;
    removeFeed: (kind: 'create' | 'update', id: string) => void;
    updateFeedName: (kind: 'create' | 'update', id: string, value: string) => void;
    updateFeedType: (kind: 'create' | 'update', id: string, value: 'RSS' | 'ATOM' | 'JSON') => void;
    updateFeedUrl: (kind: 'create' | 'update', id: string, value: string) => void;
    selectDefaultFeed: (kind: 'create' | 'update', id: string) => void;
    optionsPending?: boolean;
    options: SiteSubmissionOptionsResult;
    createProgramSelectedId?: string;
    selectProgramForCreate: (id: string) => void;
    applyProgramCustomDraftForCreate: (draft: CustomProgramDraft) => void;
    trimText: (value: string) => string;
  } = $props();

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault();
    await submitCreate();
  }
</script>

<form class="relative mt-6 space-y-6" onsubmit={handleSubmit}>
  {#if autoFillPending && autoFillTarget === 'create'}
    <div
      class="absolute inset-0 z-20 flex items-center justify-center rounded-md bg-(--color-bg)/65 backdrop-blur-[1.5px]"
    >
      <div class="flex flex-col items-center gap-2">
        <span
          class="h-6 w-6 animate-spin rounded-full border-2 border-red-700 border-t-transparent dark:border-red-400 dark:border-t-transparent"
          aria-hidden="true"
        ></span>
        <p class="text-sm text-(--color-fg-2)">自动抓取中，请稍候...</p>
      </div>
    </div>
  {/if}

  <SiteEditableFields
    mode="create"
    form={createForm}
    errors={createErrors}
    {options}
    {optionsPending}
    {inputClass}
    {textAreaClass}
    {selectClass}
    {selectChevronStyle}
    selectedProgramId={createProgramSelectedId}
    {withInputStateClass}
    isAutoFillMissing={(field) => isAutoFillMissing('create', field)}
    clearAutoFillMissing={(field) => clearAutoFillMissing('create', field)}
    fieldNeedsRefinement={(value) => fieldNeedsRefinement('create', value)}
    updateUrl={updateCreateUrl}
    applyAddressInference={() => applyAddressInference('create')}
    runAutoFill={() => runAutoFill('create')}
    addFeed={() => addFeed('create')}
    removeFeed={(id) => removeFeed('create', id)}
    updateFeedName={(id, value) => updateFeedName('create', id, value)}
    allowFeedTypeSelect={true}
    feedTypeOptions={FEED_TYPE_OPTIONS}
    updateFeedType={(id, value) => updateFeedType('create', id, value)}
    updateFeedUrl={(id, value) => updateFeedUrl('create', id, value)}
    selectDefaultFeed={(id) => selectDefaultFeed('create', id)}
    selectProgram={selectProgramForCreate}
    applyProgramCustomDraft={applyProgramCustomDraftForCreate}
    {trimText}
  />

  <div class="space-y-4 border-t border-(--color-line) pt-5">
    <p class="text-xs tracking-[0.16em] text-(--color-fg-3)">提交信息与通知</p>
    <label class="flex items-start gap-3 text-sm">
      <input
        class="h-4 w-4"
        style="accent-color: var(--color-info);"
        type="checkbox"
        bind:checked={createForm.notify_by_email}
      />
      <span class="leading-7">审核完成后通过邮件通知我结果（可选）。</span>
    </label>
    {#if createForm.notify_by_email}
      <div class="grid gap-4 md:grid-cols-2">
        <div class="space-y-2">
          <label class="block text-sm" for="create-submitter-name">提交者昵称</label>
          <input
            id="create-submitter-name"
            class={inputClass}
            bind:value={createForm.submitter_name}
            placeholder="例如：Alice"
          />
          {#if createErrors.submitter_name}
            <p class="text-xs text-(--color-fail)">{createErrors.submitter_name}</p>
          {/if}
        </div>
        <div class="space-y-2">
          <label class="block text-sm" for="create-submitter-email">提交者邮箱</label>
          <input
            id="create-submitter-email"
            class={inputClass}
            bind:value={createForm.submitter_email}
            placeholder="name@example.com"
            inputmode="email"
          />
          {#if createErrors.submitter_email}
            <p class="text-xs text-(--color-fail)">{createErrors.submitter_email}</p>
          {/if}
        </div>
      </div>
    {/if}
    <label class="flex items-start gap-3 text-sm">
      <input
        class="h-4 w-4"
        style="accent-color: var(--color-info);"
        type="checkbox"
        bind:checked={createForm.agree_terms}
      />
      <span class="leading-7"
        >我确认提交信息真实可用，并同意进入人工审核流程。<span
          class="ml-1 text-(--color-fail)"
          aria-hidden="true">✱</span
        ></span
      >
    </label>
    {#if createErrors.agree_terms}
      <p class="text-xs text-(--color-fail)">{createErrors.agree_terms}</p>
    {/if}
  </div>

  <button
    class="inline-flex min-h-11 items-center justify-center rounded-md border border-red-700/20 px-4 text-sm font-medium text-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:border-red-400/20 dark:text-red-400"
    disabled={createPending}
    type="submit"
  >
    {createPending ? '提交中...' : '提交新增申请'}
  </button>
</form>

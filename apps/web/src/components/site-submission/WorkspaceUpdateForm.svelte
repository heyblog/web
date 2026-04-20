<script lang="ts">
  import { type CustomProgramDraft } from '@/application/site-submission/site-submission.browser-form';
  import type { AutoFillFieldKey } from '@/application/site-submission/site-submission.browser-workspace';
  import {
    FEED_TYPE_OPTIONS,
    type FieldErrors,
    type SiteSubmissionOptionsResult,
    type UpdateSubmissionFormState,
  } from '@/application/site-submission/site-submission.service';

  import SiteEditableFields from './SiteEditableFields.svelte';

  let {
    autoFillPending = false,
    autoFillTarget = null,
    submitUpdate,
    updateForm,
    updateErrors = {},
    updatePending = false,
    inputClass = '',
    textAreaClass = '',
    selectClass = '',
    selectChevronStyle = '',
    withInputStateClass,
    isAutoFillMissing,
    clearAutoFillMissing,
    fieldNeedsRefinement,
    updateUpdateUrl,
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
    updateProgramSelectedId = '',
    selectProgramForUpdate,
    applyProgramCustomDraftForUpdate,
    trimText,
  }: {
    autoFillPending?: boolean;
    autoFillTarget?: 'create' | 'update' | null;
    submitUpdate: () => Promise<void>;
    updateForm: UpdateSubmissionFormState;
    updateErrors?: FieldErrors;
    updatePending?: boolean;
    inputClass?: string;
    textAreaClass?: string;
    selectClass?: string;
    selectChevronStyle?: string;
    withInputStateClass: (base: string, warned: boolean, missing: boolean) => string;
    isAutoFillMissing: (kind: 'create' | 'update', field: AutoFillFieldKey) => boolean;
    clearAutoFillMissing: (kind: 'create' | 'update', field: AutoFillFieldKey) => void;
    fieldNeedsRefinement: (kind: 'create' | 'update', value: string) => boolean;
    updateUpdateUrl: (value: string) => void;
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
    updateProgramSelectedId?: string;
    selectProgramForUpdate: (id: string) => void;
    applyProgramCustomDraftForUpdate: (draft: CustomProgramDraft) => void;
    trimText: (value: string) => string;
  } = $props();

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    await submitUpdate();
  }
</script>

<form class="relative space-y-6" onsubmit={handleSubmit}>
  {#if autoFillPending && autoFillTarget === 'update'}
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
    mode="update"
    form={updateForm}
    errors={updateErrors}
    {options}
    {optionsPending}
    {inputClass}
    {textAreaClass}
    {selectClass}
    {selectChevronStyle}
    selectedProgramId={updateProgramSelectedId}
    {withInputStateClass}
    isAutoFillMissing={(field) => isAutoFillMissing('update', field)}
    clearAutoFillMissing={(field) => clearAutoFillMissing('update', field)}
    fieldNeedsRefinement={(value) => fieldNeedsRefinement('update', value)}
    updateUrl={updateUpdateUrl}
    applyAddressInference={() => applyAddressInference('update')}
    runAutoFill={() => runAutoFill('update')}
    addFeed={() => addFeed('update')}
    removeFeed={(id) => removeFeed('update', id)}
    updateFeedName={(id, value) => updateFeedName('update', id, value)}
    allowFeedTypeSelect={true}
    feedTypeOptions={FEED_TYPE_OPTIONS}
    updateFeedType={(id, value) => updateFeedType('update', id, value)}
    updateFeedUrl={(id, value) => updateFeedUrl('update', id, value)}
    selectDefaultFeed={(id) => selectDefaultFeed('update', id)}
    selectProgram={selectProgramForUpdate}
    applyProgramCustomDraft={applyProgramCustomDraftForUpdate}
    {trimText}
  />

  <div class="space-y-4 border-t border-(--color-line) pt-5">
    <p class="text-xs tracking-[0.16em] text-(--color-fg-3)">提交信息与通知</p>
    <div class="space-y-2">
      <label class="block text-sm" for="update-reason">修改原因（可选）</label>
      <textarea
        id="update-reason"
        class={textAreaClass}
        bind:value={updateForm.submit_reason}
        placeholder="可补充说明修改背景，例如信息错误、站点改版、迁移更新等。"
      ></textarea>
      {#if updateErrors.submit_reason}
        <p class="text-xs text-(--color-fail)">{updateErrors.submit_reason}</p>
      {/if}
      {#if updateErrors.changes}<p class="text-xs text-(--color-fail)">
          {updateErrors.changes}
        </p>{/if}
    </div>

    <label class="flex items-start gap-3 text-sm">
      <input
        class="h-4 w-4"
        style="accent-color: var(--color-info);"
        type="checkbox"
        bind:checked={updateForm.notify_by_email}
      />
      <span class="leading-7">审核完成后通过邮件通知我结果（可选）。</span>
    </label>
    {#if updateForm.notify_by_email}
      <div class="grid gap-4 md:grid-cols-2">
        <div class="space-y-2">
          <label class="block text-sm" for="update-submitter-name">提交者昵称</label>
          <input
            id="update-submitter-name"
            class={inputClass}
            bind:value={updateForm.submitter_name}
          />
          {#if updateErrors.submitter_name}
            <p class="text-xs text-(--color-fail)">{updateErrors.submitter_name}</p>
          {/if}
        </div>
        <div class="space-y-2">
          <label class="block text-sm" for="update-submitter-email">提交者邮箱</label>
          <input
            id="update-submitter-email"
            class={inputClass}
            bind:value={updateForm.submitter_email}
            inputmode="email"
          />
          {#if updateErrors.submitter_email}
            <p class="text-xs text-(--color-fail)">{updateErrors.submitter_email}</p>
          {/if}
        </div>
      </div>
    {/if}
    <label class="flex items-start gap-3 text-sm">
      <input
        class="h-4 w-4"
        style="accent-color: var(--color-info);"
        type="checkbox"
        bind:checked={updateForm.agree_terms}
      />
      <span class="leading-7"
        >我已阅读并同意相关条款和声明<span class="ml-1 text-(--color-fail)" aria-hidden="true"
          >✱</span
        ></span
      >
    </label>
    {#if updateErrors.agree_terms}
      <p class="text-xs text-(--color-fail)">{updateErrors.agree_terms}</p>
    {/if}
  </div>

  <button
    class="inline-flex min-h-11 items-center justify-center rounded-md border border-red-700/20 px-4 text-sm font-medium text-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:border-red-400/20 dark:text-red-400"
    disabled={updatePending}
    type="submit"
  >
    {updatePending ? '提交中...' : '提交修订申请'}
  </button>
</form>

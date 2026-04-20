<script lang="ts">
  import type { CustomProgramDraft } from '@/application/site-submission/site-submission.browser-form';
  import type { AutoFillFieldKey } from '@/application/site-submission/site-submission.browser-workspace';
  import {
    type CreateSubmissionFormState,
    type DeleteSubmissionFormState,
    type FieldErrors,
    type QuerySubmissionFormState,
    type RestoreSubmissionFormState,
    type RestoreTargetResult,
    type SiteResolveRequest,
    type SiteResolveResult,
    type SiteSearchItem,
    type SiteSubmissionOptionsResult,
    type SubmissionPage,
    type SubmissionStatusResult,
    trimText,
    type UpdateSubmissionFormState,
  } from '@/application/site-submission/site-submission.service';

  import WorkspaceCreateForm from './WorkspaceCreateForm.svelte';
  import WorkspaceDeleteForm from './WorkspaceDeleteForm.svelte';
  import WorkspaceHeader from './WorkspaceHeader.svelte';
  import WorkspaceQueryForm from './WorkspaceQueryForm.svelte';
  import WorkspaceRestoreForm from './WorkspaceRestoreForm.svelte';
  import WorkspaceSiteResolver from './WorkspaceSiteResolver.svelte';
  import WorkspaceUpdateForm from './WorkspaceUpdateForm.svelte';

  let {
    activePage,
    autoFillPending = false,
    autoFillTarget = null,
    createForm,
    createErrors = {},
    createPending = false,
    createProgramSelectedId = '',
    deleteForm,
    deleteErrors = {},
    deletePending = false,
    restoreForm,
    restoreErrors = {},
    restorePending = false,
    restoreTarget = null,
    fieldNeedsRefinement,
    inputClass = '',
    isAutoFillMissing,
    options,
    optionsPending = false,
    queryErrors = {},
    queryForm,
    queryPending = false,
    querySuccess = null,
    resolvePending = false,
    searchError = null,
    searchPending = false,
    searchQuery = $bindable(''),
    searchResults = [],
    selectClass = '',
    selectChevronStyle = '',
    selectedSite = null,
    statusToneClass,
    textAreaClass = '',
    updateErrors = {},
    updateForm,
    updatePending = false,
    updateProgramSelectedId = '',
    withInputStateClass,
    addFeed,
    applyAddressInference,
    clearAutoFillMissing,
    removeFeed,
    applyProgramCustomDraft,
    resolveSite,
    runAutoFill,
    runSearch,
    selectDefaultFeed,
    selectProgramOption,
    submitCreate,
    submitDelete,
    submitQuery,
    submitRestore,
    submitUpdate,
    updateCreateUrl,
    updateFeedName,
    updateFeedType,
    updateFeedUrl,
    updateUpdateUrl,
  }: {
    activePage: SubmissionPage;
    autoFillPending?: boolean;
    autoFillTarget?: 'create' | 'update' | null;
    createForm: CreateSubmissionFormState;
    createErrors?: FieldErrors;
    createPending?: boolean;
    createProgramSelectedId?: string;
    deleteForm: DeleteSubmissionFormState;
    deleteErrors?: FieldErrors;
    deletePending?: boolean;
    restoreForm: RestoreSubmissionFormState;
    restoreErrors?: FieldErrors;
    restorePending?: boolean;
    restoreTarget?: RestoreTargetResult | null;
    fieldNeedsRefinement: (kind: 'create' | 'update', value: string) => boolean;
    inputClass?: string;
    isAutoFillMissing: (kind: 'create' | 'update', field: AutoFillFieldKey) => boolean;
    options: SiteSubmissionOptionsResult;
    optionsPending?: boolean;
    queryErrors?: FieldErrors;
    queryForm: QuerySubmissionFormState;
    queryPending?: boolean;
    querySuccess?: SubmissionStatusResult | null;
    resolvePending?: boolean;
    searchError?: string | null;
    searchPending?: boolean;
    searchQuery?: string;
    searchResults?: SiteSearchItem[];
    selectClass?: string;
    selectChevronStyle?: string;
    selectedSite?: SiteResolveResult | null;
    statusToneClass: (status: string) => string;
    textAreaClass?: string;
    updateErrors?: FieldErrors;
    updateForm: UpdateSubmissionFormState;
    updatePending?: boolean;
    updateProgramSelectedId?: string;
    withInputStateClass: (base: string, warned: boolean, missing: boolean) => string;
    addFeed: (kind: 'create' | 'update') => void;
    applyAddressInference: (kind: 'create' | 'update') => void;
    clearAutoFillMissing: (kind: 'create' | 'update', field: AutoFillFieldKey) => void;
    removeFeed: (kind: 'create' | 'update', id: string) => void;
    applyProgramCustomDraft: (kind: 'create' | 'update', draft: CustomProgramDraft) => void;
    resolveSite: (identifier: string | SiteResolveRequest) => Promise<void>;
    runAutoFill: (kind: 'create' | 'update') => Promise<void>;
    runSearch: () => Promise<void>;
    selectDefaultFeed: (kind: 'create' | 'update', id: string) => void;
    selectProgramOption: (kind: 'create' | 'update', id: string) => void;
    submitCreate: () => Promise<void>;
    submitDelete: () => Promise<void>;
    submitQuery: () => Promise<void>;
    submitRestore: () => Promise<void>;
    submitUpdate: () => Promise<void>;
    updateCreateUrl: (value: string) => void;
    updateFeedName: (kind: 'create' | 'update', id: string, value: string) => void;
    updateFeedType: (kind: 'create' | 'update', id: string, value: 'RSS' | 'ATOM' | 'JSON') => void;
    updateFeedUrl: (kind: 'create' | 'update', id: string, value: string) => void;
    updateUpdateUrl: (value: string) => void;
  } = $props();
</script>

<section
  class="order-2 rounded-md border border-(--color-line-med) p-5 sm:px-10 sm:py-8 lg:order-1"
>
  <WorkspaceHeader {activePage} />

  {#if activePage === 'create'}
    <div class="mt-6">
      <WorkspaceCreateForm
        {autoFillPending}
        {autoFillTarget}
        {submitCreate}
        {createForm}
        {createErrors}
        {createPending}
        {inputClass}
        {textAreaClass}
        {selectClass}
        {selectChevronStyle}
        {withInputStateClass}
        {isAutoFillMissing}
        {clearAutoFillMissing}
        {fieldNeedsRefinement}
        {updateCreateUrl}
        {applyAddressInference}
        {runAutoFill}
        {addFeed}
        {removeFeed}
        {updateFeedName}
        {updateFeedType}
        {updateFeedUrl}
        {selectDefaultFeed}
        {optionsPending}
        {options}
        {createProgramSelectedId}
        selectProgramForCreate={(id) => selectProgramOption('create', id)}
        applyProgramCustomDraftForCreate={(draft) => applyProgramCustomDraft('create', draft)}
        {trimText}
      />
    </div>
  {:else if activePage === 'update' || activePage === 'delete'}
    <div class="mt-6 space-y-6">
      <WorkspaceSiteResolver
        {inputClass}
        bind:searchQuery
        {searchPending}
        {resolvePending}
        {searchError}
        {searchResults}
        {selectedSite}
        {runSearch}
        {resolveSite}
      />

      {#if activePage === 'update' && selectedSite}
        <WorkspaceUpdateForm
          {autoFillPending}
          {autoFillTarget}
          {submitUpdate}
          {updateForm}
          {updateErrors}
          {updatePending}
          {inputClass}
          {textAreaClass}
          {selectClass}
          {selectChevronStyle}
          {withInputStateClass}
          {isAutoFillMissing}
          {clearAutoFillMissing}
          {fieldNeedsRefinement}
          {updateUpdateUrl}
          {applyAddressInference}
          {runAutoFill}
          {addFeed}
          {removeFeed}
          {updateFeedName}
          {updateFeedType}
          {updateFeedUrl}
          {selectDefaultFeed}
          {optionsPending}
          {options}
          {updateProgramSelectedId}
          selectProgramForUpdate={(id) => selectProgramOption('update', id)}
          applyProgramCustomDraftForUpdate={(draft) => applyProgramCustomDraft('update', draft)}
          {trimText}
        />
      {:else if activePage === 'update'}
        <section
          class="rounded-md border border-dashed border-(--color-line-med) p-5 text-sm text-(--color-fg-3)"
        >
          请先在上方搜索并选择需要修改的站点，加载后即可填写修订信息。
        </section>
      {/if}

      {#if activePage === 'delete' && selectedSite}
        <WorkspaceDeleteForm
          {submitDelete}
          {deleteForm}
          {deleteErrors}
          {deletePending}
          {inputClass}
          {textAreaClass}
        />
      {/if}
    </div>
  {:else if activePage === 'restore'}
    <div class="mt-6">
      <WorkspaceRestoreForm
        {submitRestore}
        target={restoreTarget}
        form={restoreForm}
        errors={restoreErrors}
        pending={restorePending}
        {inputClass}
        {textAreaClass}
      />
    </div>
  {:else}
    <WorkspaceQueryForm
      {inputClass}
      {queryForm}
      {queryErrors}
      {queryPending}
      {querySuccess}
      {submitQuery}
      {statusToneClass}
    />
  {/if}
</section>

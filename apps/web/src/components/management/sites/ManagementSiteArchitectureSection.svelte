<script lang="ts">
  import {
    type SiteSnapshotDraft,
    type SiteSubmissionOptions,
  } from '@/application/management/site-management.snapshot';
  import {
    applyProgramCustomDraftToForm,
    applyProgramOptionToForm,
    type CustomProgramDraft,
  } from '@/application/site-submission/site-submission.browser-form';
  import { trimText } from '@/application/site-submission/site-submission.service';
  import WorkspaceProgramCustomDialog from '@/components/site-submission/WorkspaceProgramCustomDialog.svelte';
  import SingleSelectCombobox from '@/shared/ui/SingleSelectCombobox.svelte';

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

  type ProgramSelectionFormState = Parameters<typeof applyProgramOptionToForm>[0];

  const asProgramSelectionFormState = (value: SiteSnapshotDraft): ProgramSelectionFormState =>
    value as unknown as ProgramSelectionFormState;

  let customProgramDialogOpen = $state(false);
  let customProgramDialogError = $state('');
  let customProgramDialogDraft = $state<CustomProgramDraft>({
    name: '',
    isOpenSource: null,
    websiteUrl: '',
    repoUrl: '',
    frameworkIds: [],
    frameworkCustomNames: [],
    languageIds: [],
    languageCustomNames: [],
  });

  const syncDraft = (): void => {
    draft = { ...draft };
  };

  const openCustomProgramDialog = (query: string) => {
    customProgramDialogDraft = {
      name: trimText(query) || trimText(draft.architecture_program_name),
      isOpenSource: draft.architecture_program_is_open_source ?? null,
      websiteUrl: draft.architecture_website_url ?? '',
      repoUrl: draft.architecture_repo_url ?? '',
      frameworkIds: [...(draft.architecture_framework_ids ?? [])],
      frameworkCustomNames: [...(draft.architecture_framework_custom_names ?? [])],
      languageIds: [...(draft.architecture_language_ids ?? [])],
      languageCustomNames: [...(draft.architecture_language_custom_names ?? [])],
    };
    customProgramDialogError = '';
    customProgramDialogOpen = true;
  };

  const closeCustomProgramDialog = () => {
    customProgramDialogOpen = false;
    customProgramDialogError = '';
  };

  const confirmCustomProgramDialog = () => {
    const normalizedValue = trimText(customProgramDialogDraft.name);

    if (!normalizedValue) {
      customProgramDialogError = '请输入程序名称后再确认。';
      return;
    }

    if (normalizedValue.length > 128) {
      customProgramDialogError = '自定义程序名称不能超过 128 个字符。';
      return;
    }

    applyProgramCustomDraftToForm(asProgramSelectionFormState(draft), {
      ...customProgramDialogDraft,
      name: normalizedValue,
    });
    syncDraft();
    closeCustomProgramDialog();
  };

  const handleSelectProgram = (optionId: string): void => {
    if (!optionId) {
      return;
    }

    applyProgramOptionToForm(asProgramSelectionFormState(draft), optionId);
    syncDraft();
  };

  let programOptions = $derived(
    (options?.programs ?? []).map((item) => ({ id: item.id, name: item.name })),
  );
  let techStackOptions = $derived(
    (options?.tech_stacks ?? []).map((item) => ({
      id: item.id,
      name: item.name,
      category: item.category,
    })),
  );
  let hasCustomProgramSelected = $derived(
    Boolean(trimText(draft.architecture_program_name)) && !trimText(draft.architecture_program_id),
  );
  let displayedProgramSelectedId = $derived(
    customProgramDialogOpen || hasCustomProgramSelected ? '' : draft.architecture_program_id,
  );
</script>

<div class="space-y-4 border-t border-(--color-line) pt-5">
  <div class="grid gap-4 md:grid-cols-2">
    <div class="space-y-2 md:col-span-2">
      {#if fieldAlerts.architecture}
        <div
          class="rounded-sm border border-[color-mix(in_srgb,var(--color-fail)_32%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-fail)_8%,transparent)] px-3 py-2"
        >
          <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改前</p>
          <p class="mt-1 text-xs whitespace-pre-wrap text-(--color-fg)">
            {fieldAlerts.architecture.value}
          </p>
        </div>
      {/if}
      <label class="block text-sm" for={`${idPrefix}-architecture-program`}>程序</label>
      <SingleSelectCombobox
        inputId={`${idPrefix}-architecture-program`}
        options={programOptions}
        selectedId={displayedProgramSelectedId}
        selectedLabel={hasCustomProgramSelected ? draft.architecture_program_name : ''}
        placeholder="输入关键词筛选程序"
        customActionLabel="使用自定义程序"
        {disabled}
        onChoose={({ id }) => handleSelectProgram(id)}
        onRequestCustom={({ query }) => openCustomProgramDialog(query)}
      />

      {#if hasCustomProgramSelected}
        <div class="mt-2 flex items-center gap-3 text-xs text-(--color-fg-2)">
          <span>
            当前使用自定义程序：<span class="font-mono">{draft.architecture_program_name}</span>
          </span>
          <button
            class="rounded-md border border-(--color-line-med) px-2 py-1 transition hover:text-(--color-fg)"
            type="button"
            onclick={() => openCustomProgramDialog('')}
            {disabled}
          >
            编辑自定义信息
          </button>
        </div>
      {/if}
    </div>
  </div>
</div>

<WorkspaceProgramCustomDialog
  bind:open={customProgramDialogOpen}
  bind:programName={customProgramDialogDraft.name}
  bind:programOpenSource={customProgramDialogDraft.isOpenSource}
  bind:websiteUrl={customProgramDialogDraft.websiteUrl}
  bind:repoUrl={customProgramDialogDraft.repoUrl}
  bind:frameworkIds={customProgramDialogDraft.frameworkIds}
  bind:frameworkCustomNames={customProgramDialogDraft.frameworkCustomNames}
  bind:languageIds={customProgramDialogDraft.languageIds}
  bind:languageCustomNames={customProgramDialogDraft.languageCustomNames}
  {techStackOptions}
  error={customProgramDialogError}
  onCancel={closeCustomProgramDialog}
  onConfirm={confirmCustomProgramDialog}
  onProgramNameInput={() => {
    customProgramDialogError = '';
  }}
/>

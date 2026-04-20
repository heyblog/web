<script lang="ts">
  import { type CustomProgramDraft } from '@/application/site-submission/site-submission.browser-form';
  import type { AutoFillFieldKey } from '@/application/site-submission/site-submission.browser-workspace';
  import type {
    FieldErrors,
    SiteSubmissionOptionsResult,
  } from '@/application/site-submission/site-submission.service';
  import SingleSelectCombobox from '@/shared/ui/SingleSelectCombobox.svelte';

  import type { CommonSiteForm } from './site-editable-fields.types';
  import WorkspaceProgramCustomDialog from './WorkspaceProgramCustomDialog.svelte';

  let {
    form = $bindable(),
    errors = {},
    options,
    optionsPending = false,
    disabled = false,
    idPrefix = 'site-fields',
    selectedProgramId = '',
    isAutoFillMissing = () => false,
    selectProgram = undefined,
    applyProgramCustomDraft = undefined,
    trimText = (value) => value.trim(),
  }: {
    form: CommonSiteForm;
    errors?: FieldErrors;
    options: SiteSubmissionOptionsResult;
    optionsPending?: boolean;
    disabled?: boolean;
    idPrefix?: string;
    selectedProgramId?: string;
    isAutoFillMissing?: (field: AutoFillFieldKey) => boolean;
    selectProgram?: ((id: string) => void) | undefined;
    applyProgramCustomDraft?: ((draft: CustomProgramDraft) => void) | undefined;
    trimText?: (value: string) => string;
  } = $props();

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

  const syncForm = (): void => {
    form = { ...form };
  };

  const openCustomProgramDialog = (query: string) => {
    customProgramDialogDraft = {
      name: trimText(query) || trimText(form.architecture_program_name),
      isOpenSource: form.architecture_program_is_open_source ?? null,
      websiteUrl: form.architecture_website_url ?? '',
      repoUrl: form.architecture_repo_url ?? '',
      frameworkIds: [...(form.architecture_framework_ids ?? [])],
      frameworkCustomNames: [...(form.architecture_framework_custom_names ?? [])],
      languageIds: [...(form.architecture_language_ids ?? [])],
      languageCustomNames: [...(form.architecture_language_custom_names ?? [])],
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

    if (applyProgramCustomDraft) {
      applyProgramCustomDraft({
        ...customProgramDialogDraft,
        name: normalizedValue,
      });
    } else {
      form.architecture_program_id = '';
      form.architecture_program_name = normalizedValue;
      form.architecture_program_is_open_source =
        typeof customProgramDialogDraft.isOpenSource === 'boolean'
          ? customProgramDialogDraft.isOpenSource
          : null;
      form.architecture_website_url = trimText(customProgramDialogDraft.websiteUrl);
      form.architecture_repo_url = trimText(customProgramDialogDraft.repoUrl);
      form.architecture_framework_ids = [...customProgramDialogDraft.frameworkIds];
      form.architecture_framework_custom_names = [...customProgramDialogDraft.frameworkCustomNames];
      form.architecture_language_ids = [...customProgramDialogDraft.languageIds];
      form.architecture_language_custom_names = [...customProgramDialogDraft.languageCustomNames];
      syncForm();
    }

    closeCustomProgramDialog();
  };

  const handleSelectProgram = (optionId: string): void => {
    if (!optionId) {
      return;
    }

    if (selectProgram) {
      selectProgram(optionId);
      return;
    }

    form.architecture_program_id = optionId;
    form.architecture_program_name = '';
    form.architecture_program_is_open_source = null;
    form.architecture_website_url = '';
    form.architecture_repo_url = '';
    form.architecture_framework_ids = [];
    form.architecture_framework_custom_names = [];
    form.architecture_language_ids = [];
    form.architecture_language_custom_names = [];
    syncForm();
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
    Boolean(trimText(form.architecture_program_name)) && !trimText(form.architecture_program_id),
  );
  let displayedProgramSelectedId = $derived(
    customProgramDialogOpen || hasCustomProgramSelected
      ? ''
      : selectedProgramId || form.architecture_program_id,
  );
</script>

<div class="space-y-4 border-t border-(--color-line) pt-5">
  <div class="grid gap-4 md:grid-cols-2">
    <div class="space-y-2 md:col-span-2">
      <label class="block text-sm" for={`${idPrefix}-architecture-program`}>程序</label>
      <div class={isAutoFillMissing('architecture') ? 'autofill-missing rounded-md' : ''}>
        <SingleSelectCombobox
          inputId={`${idPrefix}-architecture-program`}
          options={programOptions}
          selectedId={displayedProgramSelectedId}
          selectedLabel={hasCustomProgramSelected ? form.architecture_program_name : ''}
          placeholder="输入关键词筛选程序"
          customActionLabel="使用自定义程序"
          disabled={disabled || optionsPending}
          onChoose={({ id }) => handleSelectProgram(id)}
          onRequestCustom={({ query }) => openCustomProgramDialog(query)}
        />
      </div>
      {#if hasCustomProgramSelected}
        <div class="mt-2 flex items-center gap-3 text-xs text-(--color-fg-2)">
          <span>
            当前使用自定义程序：<span class="font-mono">{form.architecture_program_name}</span>
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
  {#if errors.architecture_program_name}
    <p class="text-xs text-(--color-fail)">{errors.architecture_program_name}</p>
  {/if}
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

<script lang="ts">
  import { type CustomProgramDraft } from '@/application/site-submission/site-submission.browser-form';
  import type { AutoFillFieldKey } from '@/application/site-submission/site-submission.browser-workspace';
  import type {
    FeedType,
    FieldErrors,
    SiteSubmissionOptionsResult,
  } from '@/application/site-submission/site-submission.service';

  import type { CommonSiteForm, FeedTypeOption } from './site-editable-fields.types';
  import SiteEditableArchitectureSection from './SiteEditableArchitectureSection.svelte';
  import SiteEditableBasicSection from './SiteEditableBasicSection.svelte';
  import SiteEditableFeedSection from './SiteEditableFeedSection.svelte';

  const noop = (): void => {};
  const returnFalse = (): boolean => false;
  const DEFAULT_MODE = 'update' as const;
  let {
    mode: _mode = DEFAULT_MODE,
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
    selectedProgramId = '',
    allowFeedTypeSelect = false,
    feedTypeOptions = [],
    withInputStateClass = (base) => base,
    isAutoFillMissing = returnFalse,
    clearAutoFillMissing = noop,
    fieldNeedsRefinement = returnFalse,
    updateUrl = undefined,
    applyAddressInference = undefined,
    runAutoFill = undefined,
    addFeed = undefined,
    removeFeed = undefined,
    updateFeedName = undefined,
    updateFeedUrl = undefined,
    selectDefaultFeed = undefined,
    updateFeedType = undefined,
    selectProgram = undefined,
    applyProgramCustomDraft = undefined,
    trimText = (value) => value.trim(),
  }: {
    mode?: 'create' | 'update' | 'management';
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
    selectedProgramId?: string;
    allowFeedTypeSelect?: boolean;
    feedTypeOptions?: FeedTypeOption[];
    withInputStateClass?: (base: string, warned: boolean, missing: boolean) => string;
    isAutoFillMissing?: (field: AutoFillFieldKey) => boolean;
    clearAutoFillMissing?: (field: AutoFillFieldKey) => void;
    fieldNeedsRefinement?: (value: string) => boolean;
    updateUrl?: ((value: string) => void) | undefined;
    applyAddressInference?: (() => void) | undefined;
    runAutoFill?: (() => Promise<void> | void) | undefined;
    addFeed?: (() => void) | undefined;
    removeFeed?: ((id: string) => void) | undefined;
    updateFeedName?: ((id: string, value: string) => void) | undefined;
    updateFeedUrl?: ((id: string, value: string) => void) | undefined;
    selectDefaultFeed?: ((id: string) => void) | undefined;
    updateFeedType?: ((id: string, value: FeedType) => void) | undefined;
    selectProgram?: ((id: string) => void) | undefined;
    applyProgramCustomDraft?: ((draft: CustomProgramDraft) => void) | undefined;
    trimText?: (value: string) => string;
  } = $props();
</script>

<div class="space-y-6">
  <SiteEditableBasicSection
    bind:form
    {errors}
    {options}
    {optionsPending}
    {disabled}
    {idPrefix}
    {inputClass}
    {textAreaClass}
    {selectClass}
    {selectChevronStyle}
    {withInputStateClass}
    {isAutoFillMissing}
    {clearAutoFillMissing}
    {updateUrl}
    {applyAddressInference}
    {runAutoFill}
  />

  <SiteEditableFeedSection
    bind:form
    {errors}
    {disabled}
    {idPrefix}
    {inputClass}
    {selectClass}
    {selectChevronStyle}
    {allowFeedTypeSelect}
    {feedTypeOptions}
    {withInputStateClass}
    {fieldNeedsRefinement}
    {isAutoFillMissing}
    {clearAutoFillMissing}
    {addFeed}
    {removeFeed}
    {updateFeedName}
    {updateFeedUrl}
    {selectDefaultFeed}
    {updateFeedType}
  />

  <SiteEditableArchitectureSection
    bind:form
    {errors}
    {options}
    {optionsPending}
    {disabled}
    {idPrefix}
    {selectedProgramId}
    {isAutoFillMissing}
    {selectProgram}
    {applyProgramCustomDraft}
    {trimText}
  />
</div>

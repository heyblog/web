<!-- allow: SIZE_OK — the existing multi-step form keeps field bindings and submission transitions in one island. -->
<script lang="ts">
  import { onDestroy, onMount, tick, untrack } from 'svelte';

  import {
    requestSubmissionOptions,
    requestSubmissionSnapshot,
  } from '@/api/site-submission/site-submission.browser';
  import type {
    AuditAction,
    PublicSnapshot,
    SubmissionOptions,
    SubmissionResult,
  } from '@/api/site-submission/site-submission.types';
  import { createSiteAvailability } from '@/application/site-submission/site-availability.controller.svelte';
  import {
    problemDetail,
    SiteSubmissionProblem,
    submitForm,
  } from '@/application/site-submission/site-submission.api.browser';
  import { emptySubmission } from '@/application/site-submission/site-submission.browser';
  import { applySnapshot } from '@/application/site-submission/site-submission.snapshot.browser';
  import {
    type AccountSubmissionContact,
    submissionStepCount,
    validateSubmissionStep,
  } from '@/application/site-submission/site-submission.validation';
  import InlineAlert from '@/components/feedback/InlineAlert.svelte';
  import SubmissionReceipt from '@/components/site-management/SubmissionReceipt.svelte';

  import FeedEditor from './FeedEditor.svelte';
  import ProgramPicker from './ProgramPicker.svelte';
  import SiteDetailsStep from './SiteDetailsStep.svelte';
  import SiteResolver from './SiteResolver.svelte';
  import SubmissionActions from './SubmissionActions.svelte';
  import SubmissionConfirmation from './SubmissionConfirmation.svelte';
  import SubmissionStepper from './SubmissionStepper.svelte';
  import SubmissionSuccess from './SubmissionSuccess.svelte';
  import SubmissionSummary from './SubmissionSummary.svelte';
  import TagPicker from './TagPicker.svelte';
  interface Props {
    action: AuditAction;
    initialShortId?: string;
    accountEndpoint?: string;
    initialVerifiedAddress?: string;
    accountContact?: AccountSubmissionContact;
  }
  let {
    action,
    initialShortId = '',
    accountEndpoint = '',
    accountContact,
    initialVerifiedAddress,
  }: Props = $props();
  const contact = $derived(
    accountEndpoint ? (accountContact ?? { name: '', email: null }) : undefined,
  );
  let form = $state({
    ...emptySubmission(),
    siteShortId: untrack(() => initialShortId),
  });
  let options = $state.raw<SubmissionOptions>({
    tags: [],
    components: [],
    program_dependencies: [],
    private_program_id: '',
  });
  let currentStep = $state(0);
  let furthestStep = $state(0);
  let pending = $state(false);
  let baseline = $state(untrack(() => JSON.stringify(form)));
  let dirty = $derived(JSON.stringify(form) !== baseline);
  const controller = new AbortController();
  const availability = createSiteAvailability();
  const addressContext = $derived({
    url: form.url,
    currentSiteShortID: action === 'UPDATE' ? form.siteShortId : '',
  });
  $effect(() => availability.setContext(addressContext));
  onDestroy(() => {
    controller.abort();
    availability.dispose();
  });
  let resolving = $state(untrack(() => Boolean(accountEndpoint)));
  let error = $state('');
  let showFinalValidation = $state(false);
  let result = $state.raw<SubmissionResult | null>(null);
  let detailAction = $derived(action === 'CREATE' || action === 'UPDATE');
  let labels = $derived(
    detailAction ? ['站点资料', '订阅资源', '分类程序', '确认提交'] : ['选择站点', '确认提交'],
  );
  let stepCount = $derived(submissionStepCount(action));

  onMount(async () => {
    try {
      const response = await requestSubmissionOptions();
      if (!response.ok) {
        error = await problemDetail(response);
        return;
      }
      options = (await response.json()) as SubmissionOptions;
      if (action === 'CREATE') resolving = false;
      if (initialShortId && action !== 'CREATE') await resolveSite(initialShortId);
    } catch {
      error = '资料加载失败，请刷新页面后重试。';
      resolving = Boolean(accountEndpoint);
    }
  });
  async function resolveSite(siteShortID: string): Promise<void> {
    availability.clear();
    resolving = true;
    error = '';
    const response = await requestSubmissionSnapshot(siteShortID, {
      account: Boolean(accountEndpoint),
      signal: controller.signal,
    });
    resolving = Boolean(accountEndpoint) && !response.ok;
    if (!response.ok) {
      error = await problemDetail(response);
      resolving = Boolean(accountEndpoint);
      return;
    }
    applySnapshot(form, (await response.json()) as PublicSnapshot, options);
    baseline = JSON.stringify(form);
    if (accountEndpoint && initialVerifiedAddress && siteShortID === initialShortId)
      form.url = initialVerifiedAddress;
  }
  function clearError(): void {
    error = '';
    showFinalValidation = false;
  }
  async function presentError(message: string, finalValidation = false): Promise<void> {
    error = message;
    showFinalValidation = finalValidation;
    await tick();
    document.querySelector<HTMLElement>('[data-submission-alert]')?.focus();
  }
  async function nextStep(): Promise<void> {
    if (pending || availability.checking || resolving) return;
    const validation = validateSubmissionStep(action, form, currentStep, contact);
    if (!validation.valid) {
      await presentError(validation.message);
      return;
    }
    if (detailAction && currentStep === 0 && !(await confirmAvailability())) {
      return;
    }
    clearError();
    currentStep = Math.min(currentStep + 1, stepCount - 1);
    furthestStep = Math.max(furthestStep, currentStep);
    document.querySelector<HTMLElement>('[data-submission-workspace]')?.focus();
  }
  async function confirmAvailability(force = false): Promise<boolean> {
    const available = await availability.check(addressContext, force);
    if (!available) {
      currentStep = 0;
      await tick();
      document.getElementById('site-address-availability')?.focus();
    }
    return available;
  }
  async function handleSubmit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (pending || availability.checking || resolving) return;
    for (let step = 0; step < stepCount; step += 1) {
      const validation = validateSubmissionStep(action, form, step, contact);
      if (!validation.valid) {
        currentStep = step;
        furthestStep = Math.max(furthestStep, step);
        await presentError(validation.message, step === stepCount - 1);
        return;
      }
    }
    pending = true;
    clearError();
    try {
      if (detailAction && !(await confirmAvailability(true))) return;
      result = await submitForm(action, form, { accountEndpoint, signal: controller.signal });
    } catch (caught) {
      if (
        detailAction &&
        caught instanceof SiteSubmissionProblem &&
        caught.code === 'site_address_conflict'
      ) {
        currentStep = 0;
        if (!(await confirmAvailability(true))) return;
      }
      await presentError(
        caught instanceof SiteSubmissionProblem || (!accountEndpoint && caught instanceof Error)
          ? caught.message
          : '提交失败，请稍后重试。',
      );
    } finally {
      pending = false;
    }
  }
</script>

<svelte:window
  onbeforeunload={(event) => {
    if (accountEndpoint && dirty && !result) event.preventDefault();
  }}
/>

{#if result && accountEndpoint}
  <SubmissionReceipt auditId={result.audit_id} />
{:else if result}<SubmissionSuccess {result} />{:else}
  <form class="grid min-w-0 gap-6" onsubmit={handleSubmit}>
    {#if accountEndpoint && initialVerifiedAddress}<p
        class="rounded-md bg-tint p-4 text-sm text-tint-fg"
      >
        已填入验证地址，请核对资料并提交更新申请。
      </p>{/if}
    <SubmissionStepper
      {labels}
      current={currentStep}
      furthest={furthestStep}
      onchange={(step) => {
        if (pending || availability.checking || resolving) return;
        currentStep = step;
        clearError();
      }}
    />
    <div class="grid min-w-0 items-start gap-6 lg:grid-cols-[minmax(0,1fr)_15rem]">
      <fieldset
        class="grid min-w-0 gap-6 rounded-md border border-line bg-surface p-5 sm:p-6"
        disabled={pending || availability.checking || (Boolean(accountEndpoint) && resolving)}
        aria-busy={pending || availability.checking}
        tabindex="-1"
        data-submission-workspace
      >
        <legend class="sr-only">站点申请</legend>
        {#if currentStep === 0}
          {#if action !== 'CREATE' && !accountEndpoint}<SiteResolver
              initialQuery={initialShortId}
              {resolving}
              onresolve={resolveSite}
              onclear={() => {
                availability.clear();
                form.siteShortId = '';
                furthestStep = 0;
              }}
            />{/if}
          {#if detailAction && (action === 'CREATE' || form.siteShortId)}
            <SiteDetailsStep
              bind:form
              {availability}
              purpose={action === 'CREATE' ? 'CREATE' : 'UPDATE'}
              oncheck={() => confirmAvailability(true)}
            />
          {/if}
        {:else if currentStep === 1 && detailAction}<FeedEditor bind:form />
        {:else if currentStep === 2 && detailAction}
          <TagPicker bind:form options={options.tags} />
          <div class="border-t border-line pt-6">
            <ProgramPicker
              bind:form
              options={options.components}
              dependencyRelations={options.program_dependencies}
              privateProgramID={options.private_program_id}
            />
          </div>
        {:else}
          <SubmissionConfirmation
            {action}
            accountContact={contact}
            bind:form
            validationVisible={showFinalValidation}
            onchange={clearError}
          />
        {/if}
        {#if error}<InlineAlert tone="danger">{error}</InlineAlert
          >{:else if resolving && accountEndpoint}<p class="text-sm text-fg-muted" role="status">
            正在读取站点资料…
          </p>{/if}
        <SubmissionActions
          {currentStep}
          {stepCount}
          {pending}
          checking={availability.checking}
          onnext={nextStep}
          onback={() => {
            currentStep -= 1;
            clearError();
          }}
        />
      </fieldset>
      <SubmissionSummary {form} />
    </div>
  </form>{/if}

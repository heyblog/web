<script lang="ts">
  import { onMount, tick, untrack } from 'svelte';

  import {
    problemDetail,
    SiteSubmissionProblem,
    submitForm,
  } from '@/application/site-submission/site-submission.api.browser';
  import { emptySubmission } from '@/application/site-submission/site-submission.browser';
  import { applySnapshot } from '@/application/site-submission/site-submission.snapshot.browser';
  import type {
    AuditAction,
    PublicSnapshot,
    SubmissionOptions,
    SubmissionResult,
  } from '@/application/site-submission/site-submission.types';
  import {
    submissionStepCount,
    validateSubmissionStep,
  } from '@/application/site-submission/site-submission.validation';
  import InlineAlert from '@/components/feedback/InlineAlert.svelte';

  import FeedEditor from './FeedEditor.svelte';
  import ProgramPicker from './ProgramPicker.svelte';
  import SiteDetailsStep from './SiteDetailsStep.svelte';
  import SiteResolver from './SiteResolver.svelte';
  import SubmissionConfirmation from './SubmissionConfirmation.svelte';
  import SubmissionStepper from './SubmissionStepper.svelte';
  import SubmissionSuccess from './SubmissionSuccess.svelte';
  import SubmissionSummary from './SubmissionSummary.svelte';
  import TagPicker from './TagPicker.svelte';
  interface Props {
    action: AuditAction;
    initialShortId?: string;
  }
  let { action, initialShortId = '' }: Props = $props();
  let form = $state(emptySubmission());
  let options = $state.raw<SubmissionOptions>({
    tags: [],
    components: [],
    program_dependencies: [],
    private_program_id: '',
  });
  let currentStep = $state(0);
  let furthestStep = $state(0);
  let pending = $state(false);
  let resolving = $state(false);
  let checkingSiteAddress = $state(false);
  let error = $state('');
  let showFinalValidation = $state(false);
  let result = $state.raw<SubmissionResult | null>(null);
  let siteDetailsStep = $state<{
    confirmAvailability: (force?: boolean) => Promise<boolean>;
  }>();
  let detailAction = $derived(action === 'CREATE' || action === 'UPDATE');
  let labels = $derived(
    detailAction ? ['站点资料', '订阅资源', '分类程序', '确认提交'] : ['选择站点', '确认提交'],
  );
  let stepCount = $derived(submissionStepCount(action));
  form.siteShortId = untrack(() => initialShortId);

  onMount(async () => {
    const response = await fetch('/api/site-submissions/options');
    if (!response.ok) {
      error = await problemDetail(response);
      return;
    }
    options = (await response.json()) as SubmissionOptions;
    if (initialShortId && action !== 'CREATE') await resolveSite(initialShortId);
  });
  async function resolveSite(siteShortID: string): Promise<void> {
    resolving = true;
    error = '';
    const response = await fetch(
      `/api/site-submissions/${encodeURIComponent(siteShortID)}/resolve`,
    );
    resolving = false;
    if (!response.ok) {
      error = await problemDetail(response);
      return;
    }
    applySnapshot(form, (await response.json()) as PublicSnapshot, options);
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
    const validation = validateSubmissionStep(action, form, currentStep);
    if (!validation.valid) {
      await presentError(validation.message);
      return;
    }
    if (
      action === 'CREATE' &&
      currentStep === 0 &&
      siteDetailsStep &&
      !(await siteDetailsStep.confirmAvailability())
    ) {
      return;
    }
    clearError();
    currentStep = Math.min(currentStep + 1, stepCount - 1);
    furthestStep = Math.max(furthestStep, currentStep);
    document.querySelector<HTMLElement>('[data-submission-workspace]')?.focus();
  }
  async function handleSubmit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    for (let step = 0; step < stepCount; step += 1) {
      const validation = validateSubmissionStep(action, form, step);
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
      result = await submitForm(action, form);
    } catch (caught) {
      if (
        action === 'CREATE' &&
        caught instanceof SiteSubmissionProblem &&
        caught.code === 'site_address_conflict'
      ) {
        currentStep = 0;
        await tick();
        if (siteDetailsStep && !(await siteDetailsStep.confirmAvailability(true))) return;
      }
      await presentError(caught instanceof Error ? caught.message : '提交失败，请稍后重试。');
    } finally {
      pending = false;
    }
  }
</script>

{#if result}<SubmissionSuccess {result} />{:else}
  <form class="grid min-w-0 gap-6" onsubmit={handleSubmit}>
    <SubmissionStepper
      {labels}
      current={currentStep}
      furthest={furthestStep}
      onchange={(step) => {
        currentStep = step;
        clearError();
      }}
    />
    <div class="grid min-w-0 items-start gap-6 lg:grid-cols-[minmax(0,1fr)_15rem]">
      <section
        class="grid min-w-0 gap-6 rounded-md border border-line bg-surface p-5 sm:p-6"
        tabindex="-1"
        data-submission-workspace
      >
        {#if currentStep === 0}
          {#if action !== 'CREATE'}<SiteResolver
              initialQuery={initialShortId}
              {resolving}
              onresolve={resolveSite}
            />{/if}
          {#if detailAction && (action === 'CREATE' || form.siteShortId)}
            <SiteDetailsStep
              bind:this={siteDetailsStep}
              bind:form
              checkDuplicates={action === 'CREATE'}
              oncheckingchange={(checking) => (checkingSiteAddress = checking)}
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
            bind:form
            validationVisible={showFinalValidation}
            onchange={clearError}
          />
        {/if}
        {#if error}<InlineAlert tone="danger">{error}</InlineAlert>{/if}
        <div class="flex flex-wrap justify-between gap-3 border-t border-line pt-5">
          <button
            class="min-h-11 rounded-sm border border-line-strong px-4 font-medium disabled:opacity-50"
            type="button"
            disabled={currentStep === 0}
            onclick={() => {
              currentStep -= 1;
              clearError();
            }}>上一步</button
          >{#if currentStep < stepCount - 1}<button
              class="min-h-11 rounded-sm bg-primary px-5 font-semibold text-primary-fg disabled:pointer-events-none disabled:opacity-50"
              type="button"
              disabled={checkingSiteAddress}
              onclick={nextStep}>下一步</button
            >{:else}<button
              class="min-h-11 rounded-sm bg-primary px-5 font-semibold text-primary-fg disabled:opacity-50"
              type="submit"
              disabled={pending}>{pending ? '提交中…' : '提交申请'}</button
            >{/if}
        </div>
      </section>
      <SubmissionSummary {form} />
    </div>
  </form>{/if}

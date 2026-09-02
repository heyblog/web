<script lang="ts">
  import { onMount, untrack } from 'svelte';

  import {
    applySnapshot,
    emptySubmission,
    problemDetail,
    submitForm,
    syncURLSuggestions,
  } from '@/application/site-submission/site-submission.browser';
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

  import FeedEditor from './FeedEditor.svelte';
  import ProgramPicker from './ProgramPicker.svelte';
  import SiteResolver from './SiteResolver.svelte';
  import SubmissionStepper from './SubmissionStepper.svelte';
  import SubmissionSuccess from './SubmissionSuccess.svelte';
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
  let error = $state('');
  let result = $state.raw<SubmissionResult | null>(null);
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
  function updateURL(event: Event): void {
    const input = event.currentTarget;
    if (!(input instanceof HTMLInputElement)) return;
    const previous = form.url;
    form.url = input.value;
    syncURLSuggestions(form, previous, input.value);
  }
  function nextStep(): void {
    const validation = validateSubmissionStep(action, form, currentStep);
    if (!validation.valid) {
      error = validation.message;
      return;
    }
    error = '';
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
        error = validation.message;
        return;
      }
    }
    pending = true;
    error = '';
    try {
      result = await submitForm(action, form);
    } catch (caught) {
      error = caught instanceof Error ? caught.message : '提交失败，请稍后重试。';
    } finally {
      pending = false;
    }
  }
</script>

{#if result}<SubmissionSuccess {result} />{:else}
  <form class="grid gap-6" onsubmit={handleSubmit}>
    <SubmissionStepper
      {labels}
      current={currentStep}
      furthest={furthestStep}
      onchange={(step) => {
        currentStep = step;
        error = '';
      }}
    />
    <div class="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_15rem]">
      <section
        class="grid gap-6 rounded-md border border-line bg-surface p-5 sm:p-6"
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
            <div class="grid gap-4">
              <div>
                <h2 class="text-xl font-semibold">站点资料</h2>
                <p class="mt-1 text-sm text-fg-muted">填写站点在目录中展示的基础信息。</p>
              </div>
              <label class="grid gap-2 text-sm"
                >站点名称<input
                  class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
                  bind:value={form.name}
                  maxlength="160"
                /></label
              >
              <label class="grid gap-2 text-sm"
                >主页地址<input
                  class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
                  value={form.url}
                  oninput={updateURL}
                /></label
              >
              <label class="grid gap-2 text-sm"
                >站点简介<textarea
                  class="min-h-28 rounded-sm border border-line-strong bg-surface p-3"
                  bind:value={form.summary}
                  maxlength="2000"></textarea></label
              >
            </div>
          {/if}
        {:else if currentStep === 1 && detailAction}<FeedEditor {form} />
        {:else if currentStep === 2 && detailAction}
          <TagPicker {form} options={options.tags} />
          <div class="border-t border-line pt-6">
            <ProgramPicker
              {form}
              options={options.components}
              dependencyRelations={options.program_dependencies}
              privateProgramID={options.private_program_id}
            />
          </div>
        {:else}
          <div class="grid gap-4">
            <div>
              <h2 class="text-xl font-semibold">{action === 'CREATE' ? '提交确认' : '申请说明'}</h2>
              <p class="mt-1 text-sm text-fg-muted">
                {action === 'CREATE'
                  ? '确认联系人信息，提交后进入审核。'
                  : '说明申请原因，并确认联系方式。'}
              </p>
            </div>
            {#if action !== 'CREATE'}<label class="grid gap-2 text-sm"
                >申请原因<textarea
                  class="min-h-28 rounded-sm border border-line-strong bg-surface p-3"
                  bind:value={form.reason}
                  maxlength="2000"></textarea></label
              >{/if}
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="grid gap-2 text-sm"
                >称呼（选填）<input
                  class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
                  bind:value={form.contactName}
                /></label
              ><label class="grid gap-2 text-sm"
                >邮箱（选填）<input
                  class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
                  type="email"
                  bind:value={form.contactEmail}
                /></label
              >
            </div>
            <label class="flex min-h-11 items-center gap-3 text-sm"
              ><input
                class="size-4 accent-primary"
                type="checkbox"
                bind:checked={form.notifyByEmail}
              />通过邮件接收审核结果</label
            >
          </div>
        {/if}
        {#if error}<p
            class="rounded-sm border border-danger bg-danger-bg p-3 text-sm text-danger-fg"
            role="alert"
          >
            {error}
          </p>{/if}
        <div class="flex justify-between gap-3 border-t border-line pt-5">
          <button
            class="min-h-11 rounded-sm border border-line-strong px-4 font-medium disabled:opacity-50"
            type="button"
            disabled={currentStep === 0}
            onclick={() => {
              currentStep -= 1;
              error = '';
            }}>上一步</button
          >{#if currentStep < stepCount - 1}<button
              class="min-h-11 rounded-sm bg-primary px-5 font-semibold text-primary-fg"
              type="button"
              onclick={nextStep}>下一步</button
            >{:else}<button
              class="min-h-11 rounded-sm bg-primary px-5 font-semibold text-primary-fg disabled:opacity-50"
              type="submit"
              disabled={pending}>{pending ? '提交中…' : '提交申请'}</button
            >{/if}
        </div>
      </section>
      <aside
        class="sticky top-24 hidden rounded-md border border-line bg-subtle p-4 text-sm lg:block"
      >
        <h2 class="font-semibold">申请摘要</h2>
        <dl class="mt-3 grid gap-3">
          <div>
            <dt class="text-fg">站点</dt>
            <dd class="mt-1 wrap-break-word">{form.name || '尚未填写'}</dd>
          </div>
          <div>
            <dt class="text-fg">标签</dt>
            <dd class="mt-1">{form.tags.length || '尚未选择'}</dd>
          </div>
          <div>
            <dt class="text-fg">程序</dt>
            <dd class="mt-1">{form.program.kind === 'none' ? '未填写' : form.program.name}</dd>
          </div>
        </dl>
      </aside>
    </div>
  </form>{/if}

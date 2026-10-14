<script lang="ts">
  import { onDestroy, tick, untrack } from 'svelte';

  import { requestReviewDraft } from '@/api/site-submission/site-submission.browser';
  import type {
    AuditDetail,
    ReviewSiteInput,
    SubmissionOptions,
  } from '@/api/site-submission/site-submission.types';
  import { protectReviewRequest } from '@/application/site-submission/review-navigation.browser';
  import { createSiteAvailability } from '@/application/site-submission/site-availability.controller.svelte';
  import { submissionProblem } from '@/application/site-submission/site-submission.api.browser';
  import {
    buildSubmissionPayload,
    emptySubmission,
  } from '@/application/site-submission/site-submission.browser';
  import { applySnapshot } from '@/application/site-submission/site-submission.snapshot.browser';
  import { validateSubmissionStep } from '@/application/site-submission/site-submission.validation';
  import InlineAlert from '@/components/feedback/InlineAlert.svelte';
  import FeedEditor from '@/components/site-submission/FeedEditor.svelte';
  import ProgramPicker from '@/components/site-submission/ProgramPicker.svelte';
  import SiteAddressAvailability from '@/components/site-submission/SiteAddressAvailability.svelte';
  import TagPicker from '@/components/site-submission/TagPicker.svelte';

  interface Props {
    detail: AuditDetail;
    options: SubmissionOptions;
    returnPath: string;
    canManageTaxonomy: boolean;
  }

  let { detail, options, returnPath, canManageTaxonomy }: Props = $props();
  const initialDetail = untrack(() => detail);
  const initialSnapshot = initialDetail.review_draft_snapshot ?? initialDetail.effective_snapshot;
  let form = $state(emptySubmission());
  let accessScope = $state<ReviewSiteInput['access_scope']>(
    initialSnapshot.access_scope === 'CN_ONLY' || initialSnapshot.access_scope === 'GLOBAL_ONLY'
      ? initialSnapshot.access_scope
      : 'ALL',
  );
  let pending = $state(false);
  $effect(() => protectReviewRequest(() => pending));
  let error = $state('');
  let reloadRequired = $state(false);
  let addressInput: HTMLInputElement;
  const availability = createSiteAvailability();
  const addressContext = $derived({
    url: form.url,
    currentSiteShortID: detail.action === 'UPDATE' ? detail.current_snapshot.short_id : '',
  });
  $effect(() => availability.setContext(addressContext));
  onDestroy(() => availability.dispose());

  applySnapshot(
    form,
    initialSnapshot,
    untrack(() => options),
  );

  async function save(): Promise<void> {
    if (pending || availability.checking || reloadRequired) return;
    for (let step = 0; step < 3; step += 1) {
      const result = validateSubmissionStep(detail.action, form, step);
      if (!result.valid) {
        error = result.message;
        return;
      }
    }
    pending = true;
    error = '';
    try {
      if (!(await availability.check(addressContext, true))) {
        await tick();
        document.getElementById('correction-address-availability')?.focus();
        return;
      }
      const response = await requestReviewDraft(detail.id, {
        site: { ...buildSubmissionPayload(form, detail.action).site, access_scope: accessScope },
        expected_site_revision: detail.current_snapshot.revision ?? 0,
        expected_review_draft_revision: detail.review_draft_revision,
      });
      if (!response.ok) {
        const problem = await submissionProblem(response);
        if (problem.code === 'site_address_conflict') {
          await availability.check(addressContext, true);
          await tick();
          document.getElementById('correction-address-availability')?.focus();
          error = availability.state === 'available' ? problem.message : '';
          return;
        }
        reloadRequired = response.status >= 500 || response.status === 409;
        error = problem.message;
        return;
      }
      window.location.assign(returnPath);
    } catch {
      reloadRequired = true;
      error = '连接中断，提交结果尚未确认。请重新加载核实后继续。';
    } finally {
      pending = false;
    }
  }
</script>

{#if reloadRequired}<a
    class="inline-flex min-h-11 items-center text-sm text-tint-fg underline"
    href={typeof window !== 'undefined' ? window.location.href : returnPath}>重新加载并核实结果</a
  >{/if}

<fieldset
  class="grid min-w-0 gap-6"
  disabled={pending || availability.checking}
  aria-busy={pending || availability.checking}
>
  <section class="grid min-w-0 gap-5 rounded-md border border-line bg-surface p-5 sm:p-6">
    <header class="border-b border-line pb-4">
      <p class="text-xs font-medium text-tint-fg">批准前修正</p>
      <h2 class="mt-2 text-xl font-semibold">站点资料</h2>
    </header>
    <label class="grid min-w-0 gap-2 text-sm"
      >站点名称<input
        class="min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3"
        bind:value={form.name}
        maxlength="160"
      /></label
    >
    <label class="grid min-w-0 gap-2 text-sm"
      >主页地址<input
        class="min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3"
        bind:value={form.url}
        bind:this={addressInput}
        aria-invalid={availability.state === 'duplicate' || availability.state === 'invalid'}
        aria-describedby={availability.state !== 'idle'
          ? 'correction-address-availability'
          : undefined}
        oninput={() => availability.clear()}
      /></label
    >
    <SiteAddressAvailability
      id="correction-address-availability"
      {availability}
      purpose="CORRECT"
      onretry={() => availability.check(addressContext, true)}
      onedit={() => addressInput.focus()}
    />
    <label class="grid min-w-0 gap-2 text-sm"
      >站点简介<textarea
        class="min-h-28 min-w-0 rounded-sm border border-line-strong bg-surface p-3 text-sm text-pretty sm:text-base"
        bind:value={form.summary}
        maxlength="2000"></textarea></label
    >
    <label class="grid min-w-0 gap-2 text-sm">
      访问范围
      <select
        class="min-h-11 min-w-0 rounded-md border border-line-strong bg-surface px-3 sm:min-h-10"
        bind:value={accessScope}
      >
        <option value="ALL">全球可访问</option>
        <option value="CN_ONLY">仅中国大陆可访问</option>
        <option value="GLOBAL_ONLY">仅中国大陆以外可访问</option>
      </select>
    </label>
  </section>

  <section class="min-w-0 rounded-md border border-line bg-surface p-5 sm:p-6">
    <FeedEditor bind:form />
  </section>

  <section class="grid min-w-0 gap-6 rounded-md border border-line bg-surface p-5 sm:p-6">
    <TagPicker bind:form options={options.tags} {canManageTaxonomy} disabled={pending} />
    <div class="border-t border-line pt-6">
      <ProgramPicker
        bind:form
        options={options.components}
        dependencyRelations={options.program_dependencies}
        privateProgramID={options.private_program_id}
      />
    </div>
  </section>

  {#if error}<InlineAlert tone="danger">{error}</InlineAlert>{/if}

  <div class="flex flex-wrap justify-end gap-3 border-t border-line pt-5">
    <a
      class="inline-flex min-h-11 items-center rounded-sm border border-line-strong px-4 font-medium hover:bg-subtle"
      href={returnPath}
      aria-disabled={pending}
      onclick={(event) => {
        if (pending) event.preventDefault();
      }}>取消</a
    >
    <button
      class="min-h-11 rounded-sm bg-primary px-5 font-semibold text-primary-fg disabled:opacity-50"
      type="button"
      disabled={pending || availability.checking || reloadRequired}
      onclick={save}>{pending ? '保存中…' : '保存修正并返回'}</button
    >
  </div>
</fieldset>

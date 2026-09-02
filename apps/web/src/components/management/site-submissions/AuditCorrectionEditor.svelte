<script lang="ts">
  import { untrack } from 'svelte';

  import {
    applySnapshot,
    buildSubmissionPayload,
    emptySubmission,
    problemDetail,
  } from '@/application/site-submission/site-submission.browser';
  import type {
    AuditDetail,
    SubmissionOptions,
  } from '@/application/site-submission/site-submission.types';
  import { validateSubmissionStep } from '@/application/site-submission/site-submission.validation';
  import FeedEditor from '@/components/site-submission/FeedEditor.svelte';
  import ProgramPicker from '@/components/site-submission/ProgramPicker.svelte';
  import TagPicker from '@/components/site-submission/TagPicker.svelte';

  interface Props {
    detail: AuditDetail;
    options: SubmissionOptions;
    returnPath: string;
  }

  let { detail, options, returnPath }: Props = $props();
  const initialDetail = untrack(() => detail);
  let form = $state(emptySubmission());
  let pending = $state(false);
  let error = $state('');

  applySnapshot(
    form,
    initialDetail.review_draft_snapshot ?? initialDetail.effective_snapshot,
    untrack(() => options),
  );

  async function save(): Promise<void> {
    for (let step = 0; step < 3; step += 1) {
      const result = validateSubmissionStep(detail.action, form, step);
      if (!result.valid) {
        error = result.message;
        return;
      }
    }
    pending = true;
    error = '';
    const response = await fetch(`/management/site-submissions/${detail.id}/review-draft`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        site: buildSubmissionPayload(form, detail.action).site,
        expected_site_revision: detail.current_snapshot.revision ?? 0,
        expected_review_draft_revision: detail.review_draft_revision,
      }),
    });
    pending = false;
    if (!response.ok) {
      error = await problemDetail(response);
      return;
    }
    window.location.assign(returnPath);
  }
</script>

<div class="grid gap-6">
  <section class="grid gap-5 rounded-md border border-line bg-surface p-5 sm:p-6">
    <header class="border-b border-line pb-4">
      <p class="text-xs font-medium text-tint-fg">批准前修正</p>
      <h2 class="mt-2 text-xl font-semibold">站点资料</h2>
    </header>
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
        bind:value={form.url}
      /></label
    >
    <label class="grid gap-2 text-sm"
      >站点简介<textarea
        class="min-h-28 rounded-sm border border-line-strong bg-surface p-3 text-sm text-pretty sm:text-base"
        bind:value={form.summary}
        maxlength="2000"></textarea></label
    >
  </section>

  <section class="rounded-md border border-line bg-surface p-5 sm:p-6">
    <FeedEditor {form} />
  </section>

  <section class="grid gap-6 rounded-md border border-line bg-surface p-5 sm:p-6">
    <TagPicker {form} options={options.tags} />
    <div class="border-t border-line pt-6">
      <ProgramPicker
        {form}
        options={options.components}
        dependencyRelations={options.program_dependencies}
        privateProgramID={options.private_program_id}
      />
    </div>
  </section>

  {#if error}<p
      class="rounded-sm border border-danger bg-danger-bg p-3 text-sm text-danger-fg"
      role="alert"
    >
      {error}
    </p>{/if}

  <div class="flex flex-wrap justify-end gap-3 border-t border-line pt-5">
    <a
      class="inline-flex min-h-11 items-center rounded-sm border border-line-strong px-4 font-medium hover:bg-subtle"
      href={returnPath}>取消</a
    >
    <button
      class="min-h-11 rounded-sm bg-primary px-5 font-semibold text-primary-fg disabled:opacity-50"
      type="button"
      disabled={pending}
      onclick={save}>{pending ? '保存中…' : '保存修正并返回'}</button
    >
  </div>
</div>

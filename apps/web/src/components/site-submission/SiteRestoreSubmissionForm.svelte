<script lang="ts">
  import { requestSubmissionMutation } from '@/application/site-submission/site-submission.browser-actions';
  import {
    buildSubmissionQueryHref,
    copySubmissionAuditId,
  } from '@/application/site-submission/site-submission.browser-feedback';
  import {
    buildRestoreSubmissionPayload,
    createInitialRestoreForm,
    type FieldErrors,
    type RestoreTargetResult,
    type SubmissionResult,
  } from '@/application/site-submission/site-submission.service';
  import FormMessage from '@/shared/ui/FormMessage.svelte';
  import ModalSurface from '@/shared/ui/ModalSurface.svelte';

  import {
    WORKSPACE_INPUT_CLASS,
    WORKSPACE_TEXTAREA_CLASS,
  } from './site-submission-workspace.constants';

  let {
    target = null,
  }: {
    target?: RestoreTargetResult | null;
  } = $props();

  let form = $state(createInitialRestoreForm());
  let errors = $state<FieldErrors>({});
  let pending = $state(false);
  let success = $state<SubmissionResult | null>(null);
  let copiedAuditId = $state('');

  const inputClass = WORKSPACE_INPUT_CLASS;
  const textareaClass = WORKSPACE_TEXTAREA_CLASS;

  async function submitRestore() {
    errors = {};

    if (!target) {
      errors.site_id = '恢复目标无效，请从重复提示重新进入。';
      return;
    }

    const parsed = buildRestoreSubmissionPayload(target.site_id, form);

    if (!parsed.ok) {
      errors = parsed.fieldErrors;
      return;
    }

    pending = true;

    try {
      const result = await requestSubmissionMutation({
        endpoint: '/api/site-submissions/restore',
        payload: parsed.data,
        successTitle: '恢复申请已进入审核',
        errorTitle: '恢复申请未提交',
      });

      if (result.ok) {
        success = result.data;
        form = createInitialRestoreForm();
        return;
      }

      errors = result.error.fieldErrors;
    } finally {
      pending = false;
    }
  }

  async function handleCopyAuditId(auditId: string | null) {
    if (!auditId) return;
    await copySubmissionAuditId(auditId);
    copiedAuditId = auditId;
  }

  function closeSuccessDialog() {
    success = null;
    copiedAuditId = '';
  }
</script>

{#if !target}
  <FormMessage
    tone="warning"
    eyebrow="restore"
    title="恢复入口无效"
    message="当前链接没有指向已下线站点，或该站点已经重新公开。请从重复提示重新进入恢复流程。"
  />
{:else}
  <div class="space-y-6">
    <FormMessage
      tone="info"
      eyebrow="restore target"
      title={target.name}
      message="该站点当前处于下线状态，恢复申请审核通过后会重新在前台展示。"
    >
      <div class="space-y-2 text-sm">
        <p class="break-all text-(--color-fg)">{target.url}</p>
        {#if target.bid}
          <p class="text-(--color-fg-2)">bid：{target.bid}</p>
        {/if}
        {#if target.reason}
          <p class="text-(--color-fg-2)">当前下线说明：{target.reason}</p>
        {/if}
      </div>
    </FormMessage>

    <section class="page-section space-y-5">
      <div class="grid gap-4 md:grid-cols-2">
        <div class="space-y-2">
          <label class="block text-sm" for="restore-submitter-name">联系人</label>
          <input
            id="restore-submitter-name"
            class={inputClass}
            bind:value={form.submitter_name}
            maxlength="64"
            type="text"
          />
          {#if errors.submitter_name}
            <p class="text-xs text-(--color-fail)">{errors.submitter_name}</p>
          {/if}
        </div>

        <div class="space-y-2">
          <label class="block text-sm" for="restore-submitter-email">联系邮箱</label>
          <input
            id="restore-submitter-email"
            class={inputClass}
            bind:value={form.submitter_email}
            maxlength="128"
            type="email"
          />
          {#if errors.submitter_email}
            <p class="text-xs text-(--color-fail)">{errors.submitter_email}</p>
          {/if}
        </div>
      </div>

      <div class="space-y-2">
        <label class="block text-sm" for="restore-reason">恢复说明</label>
        <textarea
          id="restore-reason"
          class={textareaClass}
          bind:value={form.restore_reason}
          placeholder="请说明为什么需要恢复该站点，以及当前状态是否已经适合重新展示。"
        ></textarea>
        {#if errors.restore_reason}
          <p class="text-xs text-(--color-fail)">{errors.restore_reason}</p>
        {/if}
      </div>

      <label class="flex items-start gap-3 text-sm text-(--color-fg-2)">
        <input bind:checked={form.notify_by_email} class="mt-1 h-4 w-4" type="checkbox" />
        <span>愿意通过邮箱接收恢复处理通知</span>
      </label>

      <label class="flex items-start gap-3 text-sm text-(--color-fg-2)">
        <input bind:checked={form.agree_terms} class="mt-1 h-4 w-4" type="checkbox" />
        <span>我确认自己了解该站点恢复后的公开展示影响，并愿意对提交说明负责。</span>
      </label>
      {#if errors.agree_terms}
        <p class="text-xs text-(--color-fail)">{errors.agree_terms}</p>
      {/if}

      {#if errors.site_id}
        <p class="text-xs text-(--color-fail)">{errors.site_id}</p>
      {/if}

      <div class="flex flex-wrap justify-end gap-3">
        <button
          class="inline-flex items-center rounded-md border border-(--color-line-med) px-4 py-2 text-sm text-(--color-fg) transition hover:border-(--color-line-strong) disabled:cursor-not-allowed disabled:opacity-60"
          disabled={pending}
          type="button"
          onclick={() => void submitRestore()}
        >
          {pending ? '提交中…' : '提交恢复申请'}
        </button>
      </div>
    </section>
  </div>
{/if}

<ModalSurface
  open={Boolean(success)}
  title="恢复申请已进入审核"
  description="请保存查询编号，后续可在查询页查看处理进度。"
  tone="info"
  confirmLabel="关闭"
  cancelLabel=""
  showCancel={false}
  showHeaderClose={true}
  headerCloseAriaLabel="关闭结果提示"
  onConfirm={closeSuccessDialog}
  onCancel={closeSuccessDialog}
>
  {#if success}
    {@const auditId = success.audit_id}
    <div class="space-y-4">
      <div class="rounded-md border border-(--color-line-med) bg-(--color-bg) px-4 py-4">
        <p class="font-mono text-[11px] tracking-[0.18em] text-(--color-info) uppercase">
          查询编号
        </p>
        <p class="mt-3 font-mono text-sm text-(--color-fg)">{auditId}</p>
      </div>

      <div class="flex flex-wrap gap-3">
        <button
          class="rounded-md border border-(--color-line-med) px-4 py-2 text-sm text-(--color-fg) transition hover:border-(--color-line-strong)"
          type="button"
          onclick={() => void handleCopyAuditId(auditId)}
        >
          {copiedAuditId === auditId ? '已复制查询 ID' : '复制查询 ID'}
        </button>
        <a
          class="inline-flex items-center rounded-md border border-(--color-line-med) px-4 py-2 text-sm text-(--color-fg) transition hover:border-(--color-line-strong)"
          href={buildSubmissionQueryHref(auditId)}
        >
          前往查询页
        </a>
      </div>
    </div>
  {/if}
</ModalSurface>

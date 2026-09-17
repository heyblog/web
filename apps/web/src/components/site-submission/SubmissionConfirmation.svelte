<script lang="ts">
  import type { EditableSubmission } from '@/application/site-submission/site-submission.browser';
  import type { AuditAction } from '@/application/site-submission/site-submission.types';

  interface Props {
    action: AuditAction;
    form: EditableSubmission;
    validationVisible: boolean;
    onchange: () => void;
  }

  let { action, form = $bindable(), validationVisible, onchange }: Props = $props();
  const contactInvalid = $derived(
    validationVisible &&
      (form.contactName.trim().length > 0 !== form.contactEmail.trim().length > 0 ||
        (form.notifyByEmail && !form.contactEmail.trim())),
  );
</script>

<div class="grid min-w-0 gap-5">
  <div>
    <h2 class="text-xl font-semibold">{action === 'CREATE' ? '提交确认' : '申请说明'}</h2>
    <p class="mt-1 text-sm text-fg-muted">
      {action === 'CREATE' ? '确认联系方式，提交后进入审核。' : '说明申请原因，并确认联系方式。'}
    </p>
  </div>

  {#if action !== 'CREATE'}
    <label class="grid min-w-0 gap-2 text-sm">
      申请原因
      <textarea
        class="min-h-28 min-w-0 rounded-sm border border-line-strong bg-surface p-3"
        bind:value={form.reason}
        maxlength="2000"
        oninput={onchange}></textarea>
    </label>
  {/if}

  <fieldset class="grid min-w-0 gap-3">
    <legend class="text-sm font-semibold">联系方式（选填）</legend>
    <p class="text-xs text-fg-muted" id="contact-requirements">
      称呼和邮箱需要同时填写或同时留空。
    </p>
    <div class="grid min-w-0 gap-4 sm:grid-cols-2">
      <label class="grid min-w-0 gap-1.5 text-sm">
        称呼
        <input
          class="min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3 sm:min-h-10"
          class:border-danger={contactInvalid}
          bind:value={form.contactName}
          maxlength="100"
          aria-invalid={contactInvalid}
          aria-describedby="contact-requirements"
          oninput={onchange}
        />
      </label>
      <label class="grid min-w-0 gap-1.5 text-sm">
        邮箱
        <input
          class="min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3 sm:min-h-10"
          class:border-danger={contactInvalid}
          type="email"
          bind:value={form.contactEmail}
          maxlength="320"
          aria-invalid={contactInvalid}
          aria-describedby="contact-requirements"
          oninput={onchange}
        />
      </label>
    </div>
  </fieldset>

  <label class="flex min-h-11 items-center gap-3 text-sm sm:min-h-10">
    <input
      class="size-4 accent-primary"
      type="checkbox"
      bind:checked={form.notifyByEmail}
      {onchange}
    />
    通过邮件接收审核结果
  </label>
</div>

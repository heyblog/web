<script lang="ts">
  import type {
    DeleteSubmissionFormState,
    FieldErrors,
  } from '@/application/site-submission/site-submission.service';

  let {
    submitDelete,
    deleteForm,
    deleteErrors = {},
    deletePending = false,
    inputClass = '',
    textAreaClass = '',
  }: {
    submitDelete: () => Promise<void>;
    deleteForm: DeleteSubmissionFormState;
    deleteErrors?: FieldErrors;
    deletePending?: boolean;
    inputClass?: string;
    textAreaClass?: string;
  } = $props();

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault();
    await submitDelete();
  }
</script>

<form class="space-y-6" onsubmit={handleSubmit}>
  <p
    class="rounded-md border border-(--color-line) px-4 py-3 text-sm leading-7 text-(--color-fg-2)"
  >
    删除操作不可逆，请谨慎提交。如果只是需要修改站点信息，请优先使用修订表单。
  </p>

  <div class="space-y-2">
    <label class="block text-sm" for="delete-reason"
      >删除原因<span class="ml-1 text-(--color-fail)" aria-hidden="true">✱</span></label
    >
    <textarea
      id="delete-reason"
      class={textAreaClass}
      bind:value={deleteForm.submit_reason}
      placeholder="请说明为什么需要删除该站点，例如失效、重复、站点已关闭等。"
    ></textarea>
    {#if deleteErrors.submit_reason}<p class="text-xs text-(--color-fail)">
        {deleteErrors.submit_reason}
      </p>{/if}
  </div>

  <div class="space-y-4 border-t border-(--color-line) pt-5">
    <p class="text-xs tracking-[0.16em] text-(--color-fg-3)">提交信息与通知</p>
    <label class="flex items-start gap-3 text-sm">
      <input class="mt-1 h-4 w-4" type="checkbox" bind:checked={deleteForm.notify_by_email} />
      <span class="leading-7">审核完成后通过邮件通知我结果（可选）。</span>
    </label>
    {#if deleteForm.notify_by_email}
      <div class="grid gap-4 md:grid-cols-2">
        <div class="space-y-2">
          <label class="block text-sm" for="delete-submitter-name">提交者昵称</label>
          <input
            id="delete-submitter-name"
            class={inputClass}
            bind:value={deleteForm.submitter_name}
          />
          {#if deleteErrors.submitter_name}<p class="text-xs text-(--color-fail)">
              {deleteErrors.submitter_name}
            </p>{/if}
        </div>
        <div class="space-y-2">
          <label class="block text-sm" for="delete-submitter-email">提交者邮箱</label>
          <input
            id="delete-submitter-email"
            class={inputClass}
            bind:value={deleteForm.submitter_email}
            inputmode="email"
          />
          {#if deleteErrors.submitter_email}<p class="text-xs text-(--color-fail)">
              {deleteErrors.submitter_email}
            </p>{/if}
        </div>
      </div>
    {/if}
    <label class="flex items-start gap-3 text-sm">
      <input class="mt-1 h-4 w-4" type="checkbox" bind:checked={deleteForm.agree_terms} />
      <span class="leading-7"
        >我确认本次删除申请真实有效，并同意进入人工审核流程。<span
          class="ml-1 text-(--color-fail)"
          aria-hidden="true">✱</span
        ></span
      >
    </label>
    {#if deleteErrors.agree_terms}<p class="text-xs text-(--color-fail)">
        {deleteErrors.agree_terms}
      </p>{/if}
  </div>
  <button
    class="inline-flex min-h-11 items-center justify-center rounded-md border border-red-700/20 px-4 text-sm font-medium text-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:border-red-400/20 dark:text-red-400"
    disabled={deletePending}
    type="submit"
  >
    {deletePending ? '提交中...' : '提交删除申请'}
  </button>
</form>

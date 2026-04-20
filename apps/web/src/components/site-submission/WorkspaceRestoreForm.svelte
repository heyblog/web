<script lang="ts">
  import type {
    FieldErrors,
    RestoreSubmissionFormState,
    RestoreTargetResult,
  } from '@/application/site-submission/site-submission.service';
  import FormMessage from '@/shared/ui/FormMessage.svelte';

  let {
    submitRestore,
    target = null,
    form,
    errors = {},
    pending = false,
    inputClass = '',
    textAreaClass = '',
  }: {
    submitRestore: () => Promise<void>;
    target?: RestoreTargetResult | null;
    form: RestoreSubmissionFormState;
    errors?: FieldErrors;
    pending?: boolean;
    inputClass?: string;
    textAreaClass?: string;
  } = $props();

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault();
    await submitRestore();
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

    <form class="space-y-5" onsubmit={handleSubmit}>
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
        <label class="block text-sm" for="restore-reason"
          >恢复说明<span class="ml-1 text-(--color-fail)" aria-hidden="true">✱</span></label
        >
        <textarea
          id="restore-reason"
          class={textAreaClass}
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

      <button
        class="inline-flex min-h-11 items-center justify-center rounded-md border border-red-700/20 px-4 text-sm font-medium text-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:border-red-400/20 dark:text-red-400"
        disabled={pending}
        type="submit"
      >
        {pending ? '提交中...' : '提交恢复申请'}
      </button>
    </form>
  </div>
{/if}

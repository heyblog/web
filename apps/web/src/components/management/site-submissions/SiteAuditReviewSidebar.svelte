<script lang="ts">
  import { WORKSPACE_TEXTAREA_CLASS } from '@/components/site-submission/site-submission-workspace.constants';
  import FormMessage from '@/shared/ui/FormMessage.svelte';

  let {
    correctionHref = null,
    isCreateOrUpdate = false,
    isPending = false,
    mode = 'detail',
    pending = false,
    formError = '',
    manualComment = '',
    onApprove,
    onReject,
  }: {
    correctionHref?: string | null;
    isCreateOrUpdate?: boolean;
    isPending?: boolean;
    mode?: 'detail' | 'process';
    pending?: boolean;
    formError?: string;
    manualComment?: string;
    onApprove: () => Promise<void> | void;
    onReject: () => Promise<void> | void;
  } = $props();
</script>

<section class="page-section space-y-4">
  <div class="space-y-2">
    <label class="block text-sm font-medium" for="audit-manual-comment">审核意见</label>
    <textarea
      id="audit-manual-comment"
      class={WORKSPACE_TEXTAREA_CLASS}
      value={manualComment}
      disabled={pending || !isPending}
      oninput={(event) => (manualComment = (event.currentTarget as HTMLTextAreaElement).value)}
      placeholder="通过可不填，驳回时必填。"
    ></textarea>
  </div>

  {#if formError}
    <FormMessage tone="error" title="提交失败" message={formError} />
  {/if}

  {#if !isPending}
    <FormMessage tone="info" title="该申请已处理" message="当前页面不再允许继续审核。" />
  {/if}

  <div class="flex flex-wrap gap-3">
    {#if mode === 'detail' && isPending && isCreateOrUpdate && correctionHref}
      <a class="action-button action-button-secondary" href={correctionHref}> 信息纠正 </a>
    {/if}

    <button
      class="action-button action-button-primary"
      type="button"
      disabled={pending || !isPending}
      onclick={() => void onApprove?.()}
    >
      {pending ? '提交中...' : '审核通过'}
    </button>

    <button
      class="action-button action-button-danger"
      type="button"
      disabled={pending || !isPending}
      onclick={() => void onReject?.()}
    >
      {pending ? '提交中...' : '审核驳回'}
    </button>
  </div>
</section>

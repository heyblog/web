<script lang="ts">
  import type { SubmissionResult } from '@/application/site-submission/site-submission.types';

  interface Props {
    result: SubmissionResult;
  }

  let { result }: Props = $props();
  let copied = $state(false);

  async function copyCredential(): Promise<void> {
    await navigator.clipboard.writeText(result.lookup_token);
    copied = true;
  }
</script>

<section class="grid gap-5 rounded-md border border-success bg-success-bg p-5" aria-live="polite">
  <div>
    <p class="text-xs font-medium text-success-fg">申请已提交</p>
    <h2 class="mt-2 text-xl font-bold">请立即保存查询凭证</h2>
  </div>
  <p class="text-sm text-fg-muted">凭证只显示这一次。遗失后无法找回，也不能仅凭审核编号查询。</p>
  <code
    class="block rounded-sm border border-line bg-surface p-3 font-mono text-sm break-all whitespace-pre-wrap"
    >{result.lookup_token}</code
  >
  <div class="flex flex-wrap gap-3">
    <button
      class="min-h-11 rounded-sm bg-primary px-4 font-medium text-primary-fg"
      type="button"
      onclick={copyCredential}>{copied ? '已复制' : '复制凭证'}</button
    >
    <a
      class="inline-flex min-h-11 items-center rounded-sm border border-line-strong px-4 font-medium"
      href="/site/submissions/query">查询审核结果</a
    >
  </div>
</section>

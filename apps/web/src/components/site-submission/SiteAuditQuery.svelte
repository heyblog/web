<script lang="ts">
  import { problemDetail } from '@/application/site-submission/site-submission.browser';
  import type { PublicAuditResult } from '@/application/site-submission/site-submission.types';

  let lookupToken = $state('');
  let pending = $state(false);
  let error = $state('');
  let result = $state.raw<PublicAuditResult | null>(null);

  async function submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    pending = true;
    error = '';
    result = null;
    const response = await fetch('/api/site-submissions/query', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ lookup_token: lookupToken.trim() }),
    });
    pending = false;
    if (!response.ok) {
      error = await problemDetail(response);
      return;
    }
    result = (await response.json()) as PublicAuditResult;
  }

  const actionLabels = { CREATE: '新增', UPDATE: '修改', DELETE: '删除', RESTORE: '恢复' } as const;
  const statusLabels = { PENDING: '待审核', APPROVED: '已通过', REJECTED: '未通过' } as const;
</script>

<form class="grid gap-6" onsubmit={submit}>
  <header class="border-b border-line pb-5">
    <p class="font-mono text-xs text-tint-fg">audit lookup</p>
    <h1 class="mt-3 text-3xl font-bold">查询审核结果</h1>
  </header>
  <label class="grid gap-2 text-sm" for="lookup-token">查询凭证</label>
  <input
    id="lookup-token"
    class="min-h-11 w-full rounded-sm border border-line-strong bg-surface px-3 font-mono text-sm"
    type="text"
    bind:value={lookupToken}
    autocomplete="off"
    autocapitalize="none"
    spellcheck="false"
    required
  />
  <button
    class="min-h-11 rounded-sm bg-primary px-5 font-semibold text-primary-fg disabled:opacity-50"
    type="submit"
    disabled={pending}>{pending ? '查询中…' : '查询'}</button
  >
  {#if error}<p
      class="rounded-sm border border-danger bg-danger-bg p-3 text-sm text-danger-fg"
      role="alert"
    >
      {error}
    </p>{/if}
</form>

{#if result}
  <section class="mt-8 grid gap-4 rounded-md border border-line p-5" aria-live="polite">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold">{actionLabels[result.action]}申请</h2>
      <span class="rounded-sm bg-subtle px-2.5 py-1 text-xs font-medium"
        >{statusLabels[result.status]}</span
      >
    </div>
    <dl class="grid gap-3 text-sm sm:grid-cols-2">
      <div>
        <dt class="text-fg-muted">提交时间</dt>
        <dd class="mt-1">{new Date(result.created_at).toLocaleString('zh-CN')}</dd>
      </div>
      {#if result.reviewed_at}<div>
          <dt class="text-fg-muted">处理时间</dt>
          <dd class="mt-1">{new Date(result.reviewed_at).toLocaleString('zh-CN')}</dd>
        </div>{/if}
    </dl>
    {#if result.reviewer_comment}<div class="border-t border-line pt-4">
        <h3 class="text-sm font-semibold">审核意见</h3>
        <p class="mt-2 text-sm whitespace-pre-wrap text-fg-muted">{result.reviewer_comment}</p>
      </div>{/if}
  </section>
{/if}

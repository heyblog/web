<script lang="ts">
  import { untrack } from 'svelte';

  import { problemDetail } from '@/application/site-submission/site-submission.browser';
  import type { AuditDetail } from '@/application/site-submission/site-submission.types';

  interface Props {
    detail: AuditDetail;
    returnPath: string;
    canManageTaxonomy: boolean;
  }

  type PendingAction = 'APPROVED' | 'REJECTED' | 'DISCARD';

  let { detail, returnPath, canManageTaxonomy }: Props = $props();
  let comment = $state(untrack(() => detail.reviewer_comment ?? ''));
  let pending = $state(false);
  let error = $state('');
  let pendingAction = $state<PendingAction | null>(null);
  let confirmDialog: HTMLDialogElement;
  let canCorrect = $derived(detail.action === 'CREATE' || detail.action === 'UPDATE');
  let approvalSnapshot = $derived(detail.review_draft_snapshot ?? detail.proposed_snapshot);
  let hasNewTaxonomy = $derived(
    approvalSnapshot.components.some((component) => !component.id) ||
      approvalSnapshot.program_dependencies.some((component) => !component.id),
  );
  let approveBlocked = $derived(hasNewTaxonomy && !canManageTaxonomy);

  function requestAction(action: PendingAction): void {
    error = '';
    if (action === 'REJECTED' && !comment.trim()) {
      error = '驳回申请时必须填写审核意见。';
      return;
    }
    pendingAction = action;
    confirmDialog.showModal();
  }

  function cancelAction(): void {
    confirmDialog.close();
    pendingAction = null;
  }

  async function confirmAction(): Promise<void> {
    if (!pendingAction) return;
    pending = true;
    error = '';
    const action = pendingAction;
    confirmDialog.close();
    const discard = action === 'DISCARD';
    const response = await fetch(
      `/management/site-submissions/${detail.id}/${discard ? 'review-draft' : 'review'}`,
      {
        method: discard ? 'DELETE' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(
          discard
            ? {
                expected_site_revision: detail.current_snapshot.revision ?? 0,
                expected_review_draft_revision: detail.review_draft_revision,
              }
            : {
                decision: action,
                reviewer_comment: comment.trim(),
                expected_site_revision: detail.current_snapshot.revision ?? 0,
                expected_review_draft_revision: detail.review_draft_revision,
              },
        ),
      },
    );
    pending = false;
    pendingAction = null;
    if (!response.ok) {
      error = await problemDetail(response);
      return;
    }
    if (discard) {
      window.location.reload();
      return;
    }
    const target = new URL(returnPath, window.location.origin);
    target.searchParams.set('reviewed', action === 'APPROVED' ? 'approved' : 'rejected');
    window.location.assign(`${target.pathname}${target.search}`);
  }
</script>

<section class="grid gap-5 rounded-md border border-line bg-surface p-5 lg:sticky lg:top-6">
  <header>
    <h2 class="text-lg font-bold">处理申请</h2>
    <p class="mt-1 text-sm text-fg-muted">批准会立即写入正式数据，驳回不会修改站点。</p>
  </header>

  {#if detail.review_draft_snapshot}<div
      class="rounded-sm border border-success bg-success-bg p-3 text-sm text-success-fg"
      role="status"
    >
      已保存批准前修正稿
    </div>{/if}

  {#if approveBlocked}<p
      class="rounded-sm border border-warning-border bg-warning-bg p-3 text-sm text-warning-fg"
      role="status"
    >
      申请包含新程序或技术。请改为已有目录项，或由拥有分类维护权限的审核者批准。
    </p>{/if}

  {#if canCorrect}<div class="grid gap-2">
      <a
        class="inline-flex min-h-11 items-center justify-center rounded-sm border border-line-strong px-4 font-medium hover:bg-subtle"
        href={`/management/site-submissions/${detail.id}/process?returnTo=${encodeURIComponent(returnPath)}`}
        >{detail.review_draft_snapshot ? '继续修正' : '进入修正'}</a
      >
      {#if detail.review_draft_snapshot}<button
          class="min-h-10 rounded-sm px-3 text-sm text-danger-fg hover:bg-danger-bg"
          type="button"
          disabled={pending}
          onclick={() => requestAction('DISCARD')}>放弃修正稿</button
        >{/if}
    </div>{/if}

  <label class="grid gap-2 text-sm"
    >审核意见<textarea
      class="min-h-28 rounded-sm border border-line-strong bg-surface p-3"
      bind:value={comment}
      disabled={pending}
      placeholder="通过可不填，驳回时必填。"></textarea></label
  >

  {#if error}<div class="grid gap-2" role="alert">
      <p class="rounded-sm border border-danger bg-danger-bg p-3 text-sm text-danger-fg">{error}</p>
      {#if error.includes('刷新')}<button
          class="min-h-10 w-fit rounded-sm border border-line-strong px-3 text-sm font-medium"
          type="button"
          onclick={() => window.location.reload()}>刷新页面</button
        >{/if}
    </div>{/if}

  <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
    <button
      class="min-h-11 rounded-sm bg-primary px-5 font-semibold text-primary-fg disabled:opacity-50"
      type="button"
      disabled={pending || approveBlocked}
      onclick={() => requestAction('APPROVED')}>批准申请</button
    >
    <button
      class="min-h-11 rounded-sm border border-danger px-5 font-semibold text-danger-fg disabled:opacity-50"
      type="button"
      disabled={pending}
      onclick={() => requestAction('REJECTED')}>驳回申请</button
    >
  </div>
</section>

<dialog
  class="m-auto w-[min(calc(100%-2rem),30rem)] rounded-md border border-line bg-surface p-0 text-fg shadow-md backdrop:bg-black/60"
  bind:this={confirmDialog}
  oncancel={cancelAction}
  aria-labelledby="audit-confirm-title"
>
  <div class="grid gap-5 p-5 sm:p-6">
    <header>
      <h2 id="audit-confirm-title" class="text-lg font-bold">
        {pendingAction === 'APPROVED'
          ? '确认批准申请？'
          : pendingAction === 'REJECTED'
            ? '确认驳回申请？'
            : '确认放弃修正稿？'}
      </h2>
      <p class="mt-2 text-sm text-fg-muted">
        {pendingAction === 'APPROVED'
          ? '批准后会立即写入正式站点数据。'
          : pendingAction === 'REJECTED'
            ? '驳回后会保留申请记录，但不会修改正式数据。'
            : '已保存的修正内容将被清除，用户原申请不会改变。'}
      </p>
    </header>
    <div class="flex justify-end gap-3">
      <button
        class="min-h-11 rounded-sm border border-line-strong px-4 font-medium"
        type="button"
        onclick={cancelAction}>取消</button
      >
      <button
        class="min-h-11 rounded-sm bg-primary px-4 font-semibold text-primary-fg"
        class:bg-danger={pendingAction === 'REJECTED' || pendingAction === 'DISCARD'}
        type="button"
        onclick={confirmAction}>确认</button
      >
    </div>
  </div>
</dialog>

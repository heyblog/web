<script lang="ts">
  import { onDestroy } from 'svelte';

  import type { Claim, OwnershipDetail } from '@/api/site-management/claims.types';
  import { formFeedback } from '@/application/forms/field-feedback.browser';
  import { createClaimReview } from '@/application/site-management/claim-review.controller.svelte';
  import { claimLabel } from '@/application/site-management/claims.model';
  import { claimMethodLabels } from '@/application/site-management/messages';
  import {
    buttonClass,
    dangerClass,
    inputClass,
    primaryClass,
  } from '@/components/management/api-keys/api-keys.styles';

  import ClaimEvidence from './ClaimEvidence.svelte';
  import OwnershipPanel from './OwnershipPanel.svelte';
  let {
    initial,
    ownership,
    reviewerId,
  }: { initial: Claim; ownership: OwnershipDetail | null; reviewerId: string } = $props();
  const controller = createClaimReview(initial, ownership);
  let reason = $state('');
  let approve = true;
  onDestroy(() => controller.dispose());
  function statusClass(status: Claim['status']) {
    return status === 'VERIFIED'
      ? 'bg-success-bg text-success-fg'
      : status === 'REJECTED' || status === 'CANCELLED'
        ? 'bg-danger-bg text-danger-fg'
        : 'bg-tint text-tint-fg';
  }
</script>

{#if controller.claim}
  {@const claim = controller.claim}
  <section class="grid max-w-4xl gap-5">
    <header class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <p class="text-xs font-semibold tracking-[0.16em] text-tint-fg uppercase">审核申请</p>
        <h2 class="mt-1 text-xl font-semibold">{claim.site_name}</h2>
        <p class="mt-1 text-sm break-all text-fg-muted">{claim.address}</p>
      </div>
      <span class="shrink-0 rounded-full px-3 py-1 text-sm {statusClass(claim.status)}"
        >{claimLabel(claim)}</span
      >
    </header>
    <div
      class="grid grid-cols-2 gap-3 rounded-lg border border-line bg-subtle p-4 text-sm sm:grid-cols-3"
    >
      <div>
        <p class="text-xs text-fg-muted">验证方式</p>
        <p class="mt-1 font-medium">{claimMethodLabels[claim.method]}</p>
      </div>
      <div>
        <p class="text-xs text-fg-muted">提交时间</p>
        <p class="mt-1">{new Date(claim.created_at).toLocaleDateString('zh-CN')}</p>
      </div>
      <div>
        <p class="text-xs text-fg-muted">申请用户</p>
        <p class="mt-1 break-all">
          {claim.applicant ? `@${claim.applicant.username}` : '账号已不可用'}
        </p>
      </div>
    </div>
    <dl class="grid gap-4 rounded-lg border border-line p-5">
      {#if claim.evidence}<div>
          <dt class="text-xs text-fg-muted">归属说明</dt>
          <dd class="mt-1 text-sm whitespace-pre-wrap">{claim.evidence}</dd>
        </div>{/if}{#if claim.evidence_url && /^https?:\/\//u.test(claim.evidence_url)}<div>
          <dt class="text-xs text-fg-muted">公开证据</dt>
          <dd class="mt-1 text-sm break-all">
            <a
              href={claim.evidence_url}
              target="_blank"
              rel="noopener noreferrer"
              class="text-tint-fg underline">{claim.evidence_url}</a
            >
          </dd>
        </div>{/if}{#if claim.review_reason}<div>
          <dt class="text-xs text-fg-muted">审核意见</dt>
          <dd class="mt-1 text-sm whitespace-pre-wrap">{claim.review_reason}</dd>
        </div>{/if}
    </dl>
    <ClaimEvidence attachments={claim.attachments ?? []} management />
    {#if controller.error}<p
        class="rounded-md bg-danger-bg p-4 text-sm text-danger-fg"
        role="alert"
      >
        {controller.error}
      </p>{/if}{#if controller.notice}<p class="text-sm text-success-fg" role="status">
        {controller.notice}
      </p>{/if}
    {#if claim.status === 'PENDING' && claim.method === 'MANUAL'}
      <section
        class="grid gap-4 rounded-lg border border-line p-5"
        aria-labelledby="review-action-heading"
      >
        <div>
          <h3 id="review-action-heading" class="text-base font-semibold">处理人工审核</h3>
          <p class="mt-1 text-sm text-fg-muted">
            填写处理意见后，选择批准或驳回。意见会保存到申请记录中。
          </p>
        </div>
        {#if claim.user_id === reviewerId}<p class="rounded-md bg-subtle p-4 text-sm">
            自己的申请需要由其他审核者处理。
          </p>{:else}<form
            use:formFeedback
            class="grid gap-4"
            onsubmit={(event) => {
              event.preventDefault();
              void controller.review(approve, reason);
            }}
          >
            <label class="grid gap-2 text-sm"
              >审核意见<textarea
                class="{inputClass} min-h-28 py-3"
                bind:value={reason}
                required
                name="reason"
                data-max-bytes="2000"
                aria-describedby="claim-review-reason-help"
                disabled={controller.busy}></textarea><span
                id="claim-review-reason-help"
                class="text-xs text-fg-muted"
                >批准或驳回均须填写审核意见，最多 2000 字节，中文和表情占多个字节。</span
              ></label
            >
            <div class="flex flex-wrap gap-3">
              <button
                type="submit"
                class={primaryClass}
                disabled={controller.busy}
                onclick={() => (approve = true)}>批准认领</button
              ><button
                type="submit"
                class={dangerClass}
                disabled={controller.busy}
                onclick={() => (approve = false)}>驳回申请</button
              >
            </div>
          </form>{/if}
      </section>
    {/if}
    <div class="flex flex-wrap items-center gap-3">
      <button
        type="button"
        class={buttonClass}
        disabled={controller.busy}
        onclick={() => void controller.refresh()}
        >{controller.busy ? '正在处理…' : '刷新最新状态'}</button
      >
      {#if controller.busy}<span class="text-sm text-fg-muted" role="status"
          >正在更新申请和归属信息…</span
        >{/if}
    </div>
    {#if controller.ownership}<OwnershipPanel
        ownership={controller.ownership}
        busy={controller.busy}
        onrevoke={(input) => controller.revoke(input)}
      />{:else}<p class="text-sm text-fg-muted">当前归属暂时无法加载，请刷新状态。</p>{/if}
    <a class="min-h-11 text-sm text-tint-fg underline" href="/management/site-claims"
      >返回认领审核</a
    >
  </section>
{/if}

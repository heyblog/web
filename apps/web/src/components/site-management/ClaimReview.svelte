<script lang="ts">
  import { onDestroy } from 'svelte';

  import type { Claim, OwnershipDetail } from '@/api/site-management/claims.types';
  import { createClaimReview } from '@/application/site-management/claim-review.controller.svelte';
  import { claimLabel } from '@/application/site-management/claims.model';
  import { claimMethodLabels } from '@/application/site-management/messages';
  import {
    buttonClass,
    dangerClass,
    inputClass,
    primaryClass,
  } from '@/components/management/api-keys/api-keys.styles';

  import OwnershipPanel from './OwnershipPanel.svelte';
  let {
    initial,
    ownership,
    reviewerId,
  }: { initial: Claim; ownership: OwnershipDetail | null; reviewerId: string } = $props();
  const controller = createClaimReview(initial, ownership);
  let reason = $state('');
  onDestroy(() => controller.dispose());
</script>

{#if controller.claim}
  {@const claim = controller.claim}
  <section class="grid max-w-4xl gap-5">
    <div class="flex flex-wrap justify-between gap-3">
      <h2 class="text-xl font-semibold">{claim.site_name}</h2>
      <span class="text-sm text-fg-muted">{claimLabel(claim)}</span>
    </div>
    <dl class="grid gap-4 rounded-md border border-line p-5">
      <div>
        <dt class="text-xs text-fg-muted">验证地址</dt>
        <dd class="mt-1 text-sm break-all">{claim.address}</dd>
      </div>
      <div>
        <dt class="text-xs text-fg-muted">申请用户</dt>
        <dd class="mt-1 text-sm">
          {claim.applicant
            ? `${claim.applicant.display_name} (@${claim.applicant.username})`
            : '账号已不可用'}
        </dd>
      </div>
      <div>
        <dt class="text-xs text-fg-muted">验证方式</dt>
        <dd class="mt-1 text-sm">{claimMethodLabels[claim.method]}</dd>
      </div>
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
    {#if controller.error}<p
        class="rounded-md bg-danger-bg p-4 text-sm text-danger-fg"
        role="alert"
      >
        {controller.error}
      </p>{/if}{#if controller.notice}<p class="text-sm text-success-fg" role="status">
        {controller.notice}
      </p>{/if}
    {#if claim.status === 'PENDING' && claim.method === 'MANUAL'}
      {#if claim.user_id === reviewerId}<p class="rounded-md bg-subtle p-4 text-sm">
          自己的申请需要由其他审核者处理。
        </p>{:else}<label class="grid gap-2 text-sm"
          >审核意见<textarea
            class="{inputClass} min-h-28 py-3"
            bind:value={reason}
            required
            maxlength="2000"
            disabled={controller.busy}></textarea></label
        >
        <div class="flex flex-wrap gap-3">
          <button
            type="button"
            class={primaryClass}
            disabled={controller.busy || !reason.trim()}
            onclick={() => void controller.review(true, reason)}>批准认领</button
          ><button
            type="button"
            class={dangerClass}
            disabled={controller.busy || !reason.trim()}
            onclick={() => void controller.review(false, reason)}>驳回申请</button
          >
        </div>{/if}
    {/if}
    <button
      type="button"
      class="{buttonClass} w-fit"
      disabled={controller.busy}
      onclick={() => void controller.refresh()}
      >{controller.busy ? '正在处理…' : '刷新最新状态'}</button
    >
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

<script lang="ts">
  import { onDestroy, onMount } from 'svelte';

  import type { Claim, ClaimChallenge } from '@/api/site-management/claims.types';
  import { createClaimDetail } from '@/application/site-management/claim-detail.controller.svelte';
  import {
    claimDestination,
    claimExpired,
    claimLabel,
  } from '@/application/site-management/claims.model';
  import { claimMethodLabels } from '@/application/site-management/messages';
  import OAuthButton from '@/components/auth/OAuthButton.svelte';
  import {
    buttonClass,
    dangerClass,
    primaryClass,
  } from '@/components/management/api-keys/api-keys.styles';

  import ClaimProof from './ClaimProof.svelte';
  let { initial, challenge }: { initial: Claim; challenge?: ClaimChallenge } = $props();
  const controller = createClaimDetail(initial, challenge);
  let now = $state(Date.now());
  let dialog = $state<HTMLDialogElement>();
  let dismiss = $state<HTMLButtonElement>();
  let action = $state<'cancel' | 'regenerate'>('regenerate');
  const loading = {
    refresh: '正在刷新…',
    check: '正在检查验证配置，最多需要 8 秒…',
    cancel: '正在取消…',
    regenerate: '正在生成新配置…',
  } as const;
  onMount(() => {
    const timer = setInterval(() => {
      now = Date.now();
    }, 1000);
    return () => clearInterval(timer);
  });
  onDestroy(() => {
    controller.dispose();
    dialog?.close();
  });
  function confirm(value: 'cancel' | 'regenerate') {
    action = value;
    dialog?.showModal();
    dismiss?.focus();
  }
</script>

{#if controller.claim}
  {@const claim = controller.claim}
  {@const destination = claimDestination(claim)}
  <section class="grid gap-5" aria-label="认领申请详情">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 id="claim-detail-heading" tabindex="-1" class="text-xl font-semibold">
          {claim.site_name}
        </h2>
        <p class="mt-2 text-sm break-all text-fg-muted">{claim.address}</p>
      </div>
      <span class="rounded-full bg-tint px-3 py-1 text-sm text-tint-fg"
        >{claimLabel(claim, now)}</span
      >
    </div>
    <p class="text-sm text-fg-muted">
      {claimMethodLabels[claim.method]} · 提交于 {new Date(claim.created_at).toLocaleString(
        'zh-CN',
      )}
    </p>
    {#if claim.expires_at}<p class="text-sm text-fg-muted">
        配置有效期至 {new Date(claim.expires_at).toLocaleString('zh-CN')}
      </p>{/if}
    {#if controller.error}<p
        class="rounded-md bg-danger-bg p-4 text-sm text-danger-fg"
        role="alert"
      >
        {controller.error}
      </p>{/if}
    {#if controller.notice}<p class="text-sm text-success-fg" role="status">
        {controller.notice}
      </p>{/if}
    {#if claim.evidence}<div>
        <h3 class="text-sm font-medium">归属说明</h3>
        <p class="mt-2 text-sm whitespace-pre-wrap">{claim.evidence}</p>
      </div>{/if}
    {#if claim.review_reason}<div>
        <h3 class="text-sm font-medium">审核意见</h3>
        <p class="mt-2 text-sm whitespace-pre-wrap">{claim.review_reason}</p>
      </div>{/if}
    {#if claim.status === 'PENDING' && claim.method === 'MANUAL'}<p
        class="rounded-md bg-subtle p-4 text-sm"
      >
        申请已提交，等待管理员审核。
      </p>
    {:else if claim.status === 'PENDING' && claim.method === 'BLOGSCLUB'}<p
        class="text-sm text-fg-muted"
      >
        授权后会核对 BlogsClub 的最新博客认证信息；尚未绑定时会先引导绑定。
      </p>
    {:else if claim.status === 'PENDING' && claimExpired(claim, now)}<p
        class="text-sm text-fg-muted"
      >
        配置已过期，请重新生成后替换站点上的配置。
      </p>
    {:else if claim.status === 'PENDING' && controller.challenge?.instructions}<ClaimProof
        challenge={controller.challenge}
      />
    {:else if claim.status === 'PENDING'}<p class="rounded-md bg-subtle p-4 text-sm">
        如果已发布验证配置，可以直接检查。需要重新取得配置时，请重新生成并替换旧配置。
      </p>{/if}
    {#if destination}<a
        class="{claim.status === 'VERIFIED' ? primaryClass : buttonClass} w-fit"
        href={destination}
        >{claim.status === 'VERIFIED' &&
        claim.address !== claim.current_address &&
        !claim.consumed_at
          ? '继续修改主页地址'
          : '管理站点'}</a
      >{:else if claim.status === 'VERIFIED'}<p class="text-sm text-fg-muted">
        验证记录已完成，当前账号暂无此站点的管理权限。
      </p>{/if}
    <div class="flex flex-wrap gap-3">
      {#if claim.can_check && claim.method === 'BLOGSCLUB'}
        <OAuthButton
          provider="blogsclub"
          intent="site-claim"
          claimId={claim.id}
          next={`/dashboard/claims/${claim.id}`}
          label="使用 BlogsClub 认证"
          disabled={controller.busy !== null}
        />
      {:else if claim.can_check}<button
          type="button"
          class={primaryClass}
          disabled={controller.busy !== null || claimExpired(claim, now)}
          onclick={() => void controller.run('check')}>检查验证</button
        >{/if}
      {#if claim.can_regenerate}<button
          type="button"
          class={buttonClass}
          disabled={controller.busy !== null}
          onclick={() => confirm('regenerate')}>重新生成配置</button
        >{/if}
      <button
        type="button"
        class={buttonClass}
        disabled={controller.busy !== null}
        onclick={() => void controller.run('refresh')}>刷新状态</button
      >
      {#if claim.can_cancel}<button
          type="button"
          class={dangerClass}
          disabled={controller.busy !== null}
          onclick={() => confirm('cancel')}>取消申请</button
        >{/if}
    </div>
    {#if controller.busy}<p class="text-sm text-fg-muted" role="status">
        {loading[controller.busy]}
      </p>{/if}
    {#if claim.status === 'REJECTED' || claim.status === 'CANCELLED'}<a
        class="{buttonClass} w-fit"
        href={`/dashboard/claims?short_id=${claim.short_id}`}>重新发起申请</a
      >{/if}
    <a class="min-h-11 text-sm text-tint-fg underline" href="/dashboard/claims">返回认领记录</a>
  </section>
{/if}
<dialog
  bind:this={dialog}
  class="m-auto max-h-[calc(100dvh-2rem)] w-[min(calc(100%-2rem),30rem)] overflow-y-auto rounded-md border border-line bg-surface p-6 text-fg shadow-md backdrop:bg-overlay"
  aria-labelledby="claim-confirm-title"
  aria-describedby="claim-confirm-description"
>
  <h2 id="claim-confirm-title" class="text-xl font-semibold">
    {action === 'regenerate' ? '重新生成验证配置？' : '取消此申请？'}
  </h2>
  <p id="claim-confirm-description" class="mt-3 text-sm/relaxed text-fg-muted">
    {action === 'regenerate'
      ? '原配置会立即失效。请使用新配置替换站点上的内容。'
      : '取消后将结束这次申请，已发布的配置也会失效。'}
  </p>
  <div class="mt-6 flex flex-wrap justify-end gap-3">
    <button bind:this={dismiss} type="button" class={buttonClass} onclick={() => dialog?.close()}
      >返回</button
    ><button
      type="button"
      class={dangerClass}
      onclick={() => {
        dialog?.close();
        void controller.run(action);
      }}>确认{action === 'regenerate' ? '重新生成' : '取消申请'}</button
    >
  </div>
</dialog>

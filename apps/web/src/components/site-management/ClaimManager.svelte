<script lang="ts">
  import { onDestroy } from 'svelte';

  import { accountRequest } from '@/api/site-management/site-management.browser';
  import {
    type Claim,
    type ClaimChallenge,
    type ClaimMethod,
    claimMethods,
    parseChallenge,
    parseClaim,
  } from '@/api/site-management/site-management.types';
  import {
    claimMethodLabels,
    claimStatusLabels,
    managementError,
    SiteManagementProblem,
  } from '@/application/site-management/messages';
  import InlineAlert from '@/components/feedback/InlineAlert.svelte';
  import SiteResolver from '@/components/site-submission/SiteResolver.svelte';
  interface Props {
    initial: readonly Claim[];
    initialShortId?: string;
  }
  let { initial, initialShortId = '' }: Props = $props();
  let claims = $state.raw(initial);
  let shortId = $state(initialShortId);
  let method = $state<ClaimMethod>('DNS_TXT');
  let address = $state('');
  let evidence = $state('');
  let evidenceURL = $state('');
  let busy = $state(false);
  let error = $state('');
  let notice = $state('');
  let challenge = $state.raw<ClaimChallenge | null>(null);
  const controller = new AbortController();
  let now = $state(Date.now());
  const expirationTimer = setInterval(() => {
    now = Date.now();
  }, 1000);
  onDestroy(() => {
    controller.abort();
    clearInterval(expirationTimer);
  });
  function expired(claim: Claim): boolean {
    return (
      claim.status === 'PENDING' &&
      claim.expires_at !== undefined &&
      Date.parse(claim.expires_at) <= now
    );
  }
  async function create(event?: SubmitEvent): Promise<void> {
    event?.preventDefault();
    if (busy) return;
    busy = true;
    error = '';
    notice = '';
    challenge = null;
    try {
      const response = await accountRequest('site-claims', {
        method: 'POST',
        body: {
          short_id: shortId,
          method,
          ...(address.trim() ? { address: address.trim() } : {}),
          ...(method === 'MANUAL'
            ? { evidence: evidence.trim(), evidence_url: evidenceURL.trim() }
            : {}),
        },
        signal: controller.signal,
      });
      if (!response.ok) throw new SiteManagementProblem(await managementError(response));
      challenge = parseChallenge(await response.json());
      claims = [challenge.claim, ...claims.filter((item) => item.id !== challenge?.claim.id)];
      notice = method === 'MANUAL' ? '人工认证申请已提交。' : '验证配置已生成。';
    } catch (caught) {
      error =
        caught instanceof SiteManagementProblem ? caught.message : '认证申请未完成，请稍后重试。';
    } finally {
      busy = false;
    }
  }
  async function operate(claim: Claim, cancel = false): Promise<void> {
    if (busy) return;
    busy = true;
    error = '';
    notice = '';
    try {
      const response = await accountRequest(`site-claims/${claim.id}${cancel ? '' : '/check'}`, {
        method: cancel ? 'DELETE' : 'POST',
        signal: controller.signal,
      });
      if (!response.ok) throw new SiteManagementProblem(await managementError(response));
      const updated = parseClaim(await response.json());
      claims = claims.map((item) => (item.id === updated.id ? updated : item));
      notice = cancel
        ? '申请已取消。'
        : updated.status === 'VERIFIED'
          ? '站点认证成功。'
          : '尚未找到匹配的验证配置。';
      if (cancel || updated.status === 'VERIFIED') challenge = null;
    } catch (caught) {
      error = caught instanceof SiteManagementProblem ? caught.message : '验证未完成，请稍后重试。';
    } finally {
      busy = false;
    }
  }
</script>

<div class="grid gap-8">
  <form class="grid gap-5 border-b border-line pb-8" onsubmit={create}>
    <h2 class="text-xl font-semibold">认证站点</h2>
    <SiteResolver
      initialQuery={initialShortId}
      resolving={busy}
      onresolve={async (id) => {
        shortId = id;
      }}
    />
    {#if shortId}<p class="text-sm text-fg-muted">所选站点：{shortId}</p>{/if}
    <a
      class="inline-flex min-h-11 items-center text-sm text-tint-fg underline"
      href="/dashboard/sites/new">站点未收录，提交新增申请</a
    >
    <label class="grid gap-2 text-sm"
      >验证地址（可选）<input
        class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
        type="url"
        bind:value={address}
        placeholder="https://example.com/"
      /></label
    >
    <fieldset class="grid gap-3">
      <legend class="mb-2 text-sm font-medium">认证方式</legend>
      <div class="flex flex-wrap gap-5">
        {#each claimMethods as value (value)}<label class="flex min-h-11 items-center gap-2 text-sm"
            ><input type="radio" bind:group={method} {value} />{claimMethodLabels[value]}</label
          >{/each}
      </div>
    </fieldset>
    {#if method === 'MANUAL'}
      <label class="grid gap-2 text-sm"
        >归属说明<textarea
          class="min-h-28 rounded-sm border border-line-strong bg-surface p-3"
          bind:value={evidence}
          required
          maxlength="4000"></textarea></label
      >
      <label class="grid gap-2 text-sm"
        >公开证据链接<input
          class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
          type="url"
          bind:value={evidenceURL}
        /></label
      >
    {/if}
    <button
      class="min-h-11 w-fit rounded-sm bg-primary px-5 font-semibold text-primary-fg disabled:opacity-50"
      type="submit"
      disabled={busy || !shortId}>{busy ? '处理中…' : '提交认证申请'}</button
    >
  </form>
  {#if error}<InlineAlert tone="danger">{error}</InlineAlert>{/if}
  {#if notice}<p class="text-sm text-success-fg" role="status">{notice}</p>{/if}
  {#if challenge?.instructions}
    <section class="grid min-w-0 gap-4 border-b border-line pb-6" aria-label="验证配置">
      <h2 class="text-xl font-semibold">验证配置</h2>
      {#if challenge.claim.method === 'DNS_TXT'}<dl class="grid gap-3">
          <div>
            <dt class="text-sm text-fg-muted">TXT 记录名</dt>
            <dd class="mt-1 font-mono text-sm break-all">{challenge.instructions.dns_name}</dd>
          </div>
          <div>
            <dt class="text-sm text-fg-muted">TXT 记录值</dt>
            <dd class="mt-1 font-mono text-sm break-all">{challenge.instructions.dns_value}</dd>
          </div>
        </dl>
      {:else if challenge.claim.method === 'META'}<label class="grid gap-2 text-sm"
          >首页 head 中的 meta 标签<textarea
            class="min-h-24 rounded-sm border border-line-strong bg-surface p-3 font-mono"
            readonly
            value={challenge.instructions.meta}></textarea></label
        >
      {:else if challenge.claim.method === 'FILE'}<p class="text-sm break-all">
          {challenge.instructions.file_url}
        </p>
        <label class="grid gap-2 text-sm"
          >文件内容<textarea
            class="min-h-24 rounded-sm border border-line-strong bg-surface p-3 font-mono"
            readonly
            value={challenge.instructions.file_content}></textarea></label
        >{/if}
      <p class="text-sm text-fg-muted">
        有效期至 {challenge.claim.expires_at
          ? new Date(challenge.claim.expires_at).toLocaleString('zh-CN')
          : '本次验证完成'}。离开页面后可重新生成验证配置。
      </p>
      <button
        class="min-h-11 w-fit rounded-sm border border-line-strong px-4 font-medium"
        type="button"
        disabled={busy || expired(challenge.claim)}
        onclick={() => challenge && operate(challenge.claim)}>检查验证</button
      >
    </section>
  {/if}
  <section>
    <h2 class="mb-4 text-xl font-semibold">认证记录</h2>
    {#if claims.length === 0}<p class="py-6 text-sm text-fg-muted">暂无认证申请。</p>{/if}
    <ul class="divide-y divide-line">
      {#each claims as claim (claim.id)}<li class="grid gap-3 py-5">
          <div class="flex flex-wrap justify-between gap-3">
            <strong class="min-w-0 text-sm break-all">{claim.address}</strong><span
              class="text-sm text-fg-muted"
              >{expired(claim) ? '已过期' : claimStatusLabels[claim.status]}</span
            >
          </div>
          <p class="text-sm text-fg-muted">
            {claimMethodLabels[claim.method]} · {new Date(claim.created_at).toLocaleString('zh-CN')}
          </p>
          {#if claim.review_reason}<p class="text-sm whitespace-pre-wrap">
              {claim.review_reason}
            </p>{/if}{#if claim.status === 'VERIFIED'}<a
              class="text-sm text-tint-fg underline"
              href={`/dashboard/sites/${claim.short_id}`}>管理站点</a
            >{:else if claim.status === 'PENDING'}<div class="flex flex-wrap gap-3">
              {#if claim.method !== 'MANUAL'}<button
                  class="min-h-11 rounded-sm border border-line-strong px-4 text-sm"
                  type="button"
                  disabled={busy || expired(claim)}
                  onclick={() => operate(claim)}>检查验证</button
                >
                <button
                  class="min-h-11 rounded-sm border border-line-strong px-4 text-sm"
                  type="button"
                  disabled={busy}
                  onclick={() => {
                    shortId = claim.short_id;
                    address = claim.address;
                    method = claim.method;
                    return create();
                  }}>重新生成验证配置</button
                >
              {/if}<button
                class="min-h-11 rounded-sm border border-line-strong px-4 text-sm"
                type="button"
                disabled={busy}
                onclick={() => operate(claim, true)}>取消申请</button
              >
            </div>{/if}
        </li>{/each}
    </ul>
  </section>
</div>

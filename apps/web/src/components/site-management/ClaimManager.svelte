<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';

  import type { OAuthRequest } from '@/api/auth/oauth.types';
  import { type ClaimMethod, claimMethods } from '@/api/site-management/claims.types';
  import { createClaimCreator } from '@/application/site-management/claim-create.controller.svelte';
  import { claimCanCreate, claimLabel } from '@/application/site-management/claims.model';
  import { claimMethodLabels } from '@/application/site-management/messages';
  import OAuthButton from '@/components/auth/OAuthButton.svelte';
  import {
    buttonClass,
    inputClass,
    primaryClass,
  } from '@/components/management/api-keys/api-keys.styles';
  import SiteAddressAvailability from '@/components/site-submission/SiteAddressAvailability.svelte';
  import SiteResolver from '@/components/site-submission/SiteResolver.svelte';

  import ClaimDetail from './ClaimDetail.svelte';
  let {
    initialShortId = '',
    purpose = 'claim',
  }: { initialShortId?: string; purpose?: 'claim' | 'address' } = $props();
  const controller = createClaimCreator();
  let method = $state<ClaimMethod>('BLOGSCLUB');
  let address = $state('');
  let addressInput = $state<HTMLInputElement>();
  let evidence = $state('');
  let evidenceURL = $state('');
  let quickAuth = $state<{ authorize(): void }>();
  const newAddress = $derived(purpose === 'address');
  const addressContext = $derived({
    url: address,
    currentSiteShortID: controller.context?.site.short_id,
  });
  $effect(() => controller.availability.setContext(addressContext));
  onMount(() => {
    if (initialShortId) void controller.select(initialShortId);
  });
  onDestroy(() => controller.dispose());
  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (method === 'BLOGSCLUB') {
      quickAuth?.authorize();
      return;
    }
    await controller.create({ method, newAddress, address, evidence, evidenceURL });
    if (newAddress && !controller.created && controller.availability.state !== 'idle') {
      await tick();
      document.getElementById('claim-address-availability')?.focus();
    }
    if (controller.created) {
      history.replaceState(null, '', `/dashboard/claims/${controller.created.claim.id}`);
      await tick();
      document.getElementById('claim-detail-heading')?.focus();
    }
  }
  async function prepareQuickVerification(): Promise<OAuthRequest | null> {
    const result = await controller.create(
      { method: 'BLOGSCLUB', newAddress, address, evidence: '', evidenceURL: '' },
      false,
    );
    const claim = result?.claim;
    if (!claim) return null;
    history.replaceState(null, '', `/dashboard/claims/${claim.id}`);
    return {
      provider: 'blogsclub',
      intent: 'site-claim',
      claimId: claim.id,
      next: `/dashboard/claims/${claim.id}`,
    };
  }
</script>

{#if controller.created}
  <ClaimDetail initial={controller.created.claim} challenge={controller.created} />
{:else}
  <form class="grid gap-5" onsubmit={submit}>
    <h2 class="text-xl font-semibold">{newAddress ? '验证新地址' : '认领站点'}</h2>
    <SiteResolver
      initialQuery={initialShortId}
      resolving={controller.busy}
      onresolve={(id) => controller.select(id)}
      onclear={() => controller.clear()}
    />
    {#if controller.busy}<p class="text-sm text-fg-muted" role="status">正在处理…</p>{/if}
    {#if controller.error}<p
        class="rounded-md bg-danger-bg p-4 text-sm text-danger-fg"
        role="alert"
      >
        {controller.error}
      </p>{/if}
    {#if controller.context}
      {@const context = controller.context}
      <div class="grid gap-2 rounded-md border border-line bg-subtle p-4">
        <strong class="text-sm">{context.site.name}</strong>
        <p class="text-sm break-all text-fg-muted">{context.site.address}</p>
        {#if context.ownership_state === 'other'}<p class="text-sm">
            此站点已有所有者。
          </p>{:else if context.can_manage}<a
            class="min-h-11 text-sm text-tint-fg underline"
            href={`/dashboard/sites/${context.site.short_id}`}>管理我的站点</a
          >{:else if context.ownership_state === 'stale'}<p class="text-sm">
            请重新验证当前主页地址以恢复管理权限。
          </p>{/if}
      </div>
      {#if context.pending_claims.length}<section class="grid gap-2" aria-label="进行中的申请">
          <h3 class="text-sm font-semibold">继续已有申请</h3>
          {#each context.pending_claims as claim (claim.id)}<a
              class="min-h-11 rounded-md border border-line p-3 text-sm text-tint-fg"
              href={`/dashboard/claims/${claim.id}`}
              >{claimMethodLabels[claim.method]} · {claimLabel(claim)}<span
                class="mt-1 block break-all text-fg-muted">{claim.address}</span
              ></a
            >{/each}
        </section>{/if}
      {#if claimCanCreate(context, newAddress)}
        {#if newAddress}<label class="grid gap-2 text-sm"
            >新的主页地址<input
              class={inputClass}
              type="url"
              bind:this={addressInput}
              bind:value={address}
              aria-invalid={controller.availability.state === 'duplicate' ||
                controller.availability.state === 'invalid'}
              aria-describedby={controller.availability.state !== 'idle'
                ? 'claim-address-availability'
                : undefined}
              oninput={() => controller.availability.clear()}
              required
              disabled={controller.busy}
            /></label
          >
          <SiteAddressAvailability
            id="claim-address-availability"
            availability={controller.availability}
            purpose="VERIFY"
            onretry={() => controller.availability.check(addressContext, true)}
            onedit={() => addressInput?.focus()}
          />{/if}
        <fieldset class="grid gap-3" disabled={controller.busy}>
          <legend class="mb-2 text-sm font-medium">验证方式</legend>
          <div class="flex flex-wrap gap-5">
            {#each claimMethods as value (value)}<label
                class="flex min-h-11 items-center gap-2 text-sm"
                ><input type="radio" bind:group={method} {value} />{claimMethodLabels[value]}</label
              >{/each}
          </div>
        </fieldset>
        {#if method === 'MANUAL'}<label class="grid gap-2 text-sm"
            >归属说明<textarea
              class="{inputClass} min-h-28 py-3"
              bind:value={evidence}
              required
              maxlength="4000"
              disabled={controller.busy}></textarea></label
          ><label class="grid gap-2 text-sm"
            >公开证据链接（可选）<input
              class={inputClass}
              type="url"
              bind:value={evidenceURL}
              maxlength="2048"
              disabled={controller.busy}
            /></label
          >{/if}
        {#if method === 'BLOGSCLUB'}
          <p class="text-sm text-fg-muted">
            使用 BlogsClub 已认证的博客地址验证；尚未绑定时会先引导绑定。
          </p>
          <OAuthButton
            bind:this={quickAuth}
            provider="blogsclub"
            intent="site-claim"
            label="使用 BlogsClub 快速认证"
            disabled={controller.busy || (newAddress && !URL.canParse(address))}
            prepare={prepareQuickVerification}
          />
        {:else}<button
            class={primaryClass}
            type="submit"
            disabled={controller.busy || controller.availability.checking}
            >{method === 'MANUAL' ? '提交人工审核' : '生成验证配置'}</button
          >{/if}
      {/if}
    {/if}
    <a class="{buttonClass} w-fit" href="/dashboard/sites/new">站点未收录，提交新增申请</a>
  </form>
{/if}

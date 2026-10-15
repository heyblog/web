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
  const step = $derived(controller.context ? 2 : 1);
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
  <form class="grid gap-6" onsubmit={submit}>
    <header class="grid gap-2">
      <p class="text-xs font-semibold tracking-[0.16em] text-tint-fg uppercase">站点认证流程</p>
      <h2 class="text-xl font-semibold">{newAddress ? '验证新地址' : '认领站点'}</h2>
      <p class="max-w-2xl text-sm/6 text-fg-muted">
        先选择要认证的站点，再选择认证方式。BlogsClub
        快速认证会在授权窗口中完成，人工审核适合暂时无法使用自动认证的情况。
      </p>
    </header>

    <ol class="grid grid-cols-3 gap-2" aria-label="认证进度">
      {#each ['选择站点', '选择方式', '完成认证'] as label, index (label)}
        {@const current = index + 1 === step}
        <li class="grid gap-2 border-t-2 pt-3 {index + 1 <= step ? 'border-tint' : 'border-line'}">
          <span class="text-xs font-semibold {current ? 'text-tint-fg' : 'text-fg-muted'}"
            >0{index + 1}</span
          >
          <span class="text-sm {current ? 'font-medium text-fg' : 'text-fg-muted'}">{label}</span>
        </li>
      {/each}
    </ol>

    <section class="grid gap-3" aria-labelledby="claim-site-heading">
      <div>
        <h3 id="claim-site-heading" class="text-base font-semibold">1. 选择站点</h3>
        <p class="mt-1 text-sm text-fg-muted">输入站点名称、短 ID 或主页地址查找公开站点。</p>
      </div>
      <SiteResolver
        initialQuery={initialShortId}
        resolving={controller.busy}
        onresolve={(id) => controller.select(id)}
        onclear={() => controller.clear()}
      />
      {#if controller.busy}<p class="text-sm text-fg-muted" role="status">正在处理…</p>{/if}
      {#if controller.error}<p
          class="rounded-md border border-danger bg-danger-bg p-4 text-sm text-danger-fg"
          role="alert"
        >
          {controller.error}
        </p>{/if}
    </section>

    {#if controller.context}
      {@const context = controller.context}
      <section class="grid gap-5" aria-labelledby="claim-method-heading">
        <div
          class="grid gap-2 rounded-lg border border-line bg-subtle p-4 sm:flex sm:items-start sm:justify-between sm:gap-6"
        >
          <div class="min-w-0">
            <p class="text-xs font-semibold tracking-wide text-fg-muted uppercase">已选择站点</p>
            <strong class="mt-1 block text-base">{context.site.name}</strong>
            <p class="mt-1 text-sm break-all text-fg-muted">{context.site.address}</p>
          </div>
          <div class="shrink-0 text-sm sm:text-right">
            {#if context.ownership_state === 'other'}<p class="text-warning-fg">
                此站点已有所有者。
              </p>
            {:else if context.can_manage}<a
                class="text-tint-fg underline"
                href={`/dashboard/sites/${context.site.short_id}`}>管理我的站点</a
              >
            {:else if context.ownership_state === 'stale'}<p class="text-warning-fg">
                请重新验证当前主页地址以恢复管理权限。
              </p>{/if}
          </div>
        </div>

        {#if context.pending_claims.length}
          <div class="grid gap-3" aria-label="进行中的申请">
            <div>
              <h3 class="text-sm font-semibold">继续已有申请</h3>
              <p class="mt-1 text-sm text-fg-muted">你可以直接打开下面的申请，继续上次的认证。</p>
            </div>
            <div class="grid gap-2">
              {#each context.pending_claims as claim (claim.id)}
                <a
                  class="grid min-h-11 gap-1 rounded-md border border-line p-3 text-sm transition-colors hover:border-line-strong hover:bg-subtle"
                  href={`/dashboard/claims/${claim.id}`}
                >
                  <span class="font-medium text-tint-fg"
                    >{claimMethodLabels[claim.method]} · {claimLabel(claim)}</span
                  >
                  <span class="break-all text-fg-muted">{claim.address}</span>
                </a>
              {/each}
            </div>
          </div>
        {/if}

        {#if claimCanCreate(context, newAddress)}
          {#if newAddress}
            <label class="grid gap-2 text-sm">
              <span class="font-medium">新的主页地址</span>
              <input
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
                placeholder="https://example.com/"
              />
              <span class="text-xs text-fg-muted">请输入准备认证的完整 HTTP 或 HTTPS 地址。</span>
            </label>
            <SiteAddressAvailability
              id="claim-address-availability"
              availability={controller.availability}
              purpose="VERIFY"
              onretry={() => controller.availability.check(addressContext, true)}
              onedit={() => addressInput?.focus()}
            />
          {/if}

          <fieldset class="grid gap-3" disabled={controller.busy}>
            <legend id="claim-method-heading" class="text-base font-semibold"
              >2. 选择认证方式</legend
            >
            <div class="grid gap-3 sm:grid-cols-2">
              {#each claimMethods as value (value)}
                <label
                  class="relative grid min-h-11 cursor-pointer gap-1 rounded-lg border p-4 transition-colors {method ===
                  value
                    ? 'border-tint bg-tint/10'
                    : 'border-line hover:border-line-strong'} focus-within:ring-2 focus-within:ring-focus/20"
                >
                  <input class="sr-only" type="radio" bind:group={method} {value} />
                  <span class="text-sm font-medium">{claimMethodLabels[value]}</span>
                  <span class="text-xs/5 text-pretty text-fg-muted">
                    {value === 'BLOGSCLUB'
                      ? '授权后自动核对已认证的博客地址。'
                      : value === 'MANUAL'
                        ? '提交说明和公开证据，由管理员审核。'
                        : '生成验证配置并发布到站点后检查。'}
                  </span>
                </label>
              {/each}
            </div>
          </fieldset>

          {#if method === 'MANUAL'}
            <div class="grid gap-4 rounded-lg border border-line p-4">
              <label class="grid gap-2 text-sm">
                <span class="font-medium">归属说明</span>
                <textarea
                  class="{inputClass} min-h-28 py-3"
                  bind:value={evidence}
                  required
                  maxlength="4000"
                  disabled={controller.busy}
                  placeholder="说明你与该站点的关系，以及管理员可以核对的线索。"></textarea>
              </label>
              <label class="grid gap-2 text-sm">
                <span class="font-medium">公开证据链接（可选）</span>
                <input
                  class={inputClass}
                  type="url"
                  bind:value={evidenceURL}
                  maxlength="2048"
                  disabled={controller.busy}
                  placeholder="https://example.com/about"
                />
              </label>
            </div>
          {:else if method === 'BLOGSCLUB'}
            <div class="grid gap-3 rounded-lg border border-blogsclub/30 bg-blogsclub/5 p-4">
              <div>
                <h3 class="font-medium text-blogsclub">BlogsClub 快速认证</h3>
                <p class="mt-1 text-sm/6 text-fg-muted">通过 BlogsClub 账号快速认证站点。</p>
              </div>
              <OAuthButton
                bind:this={quickAuth}
                provider="blogsclub"
                intent="site-claim"
                label="使用 BlogsClub 快速认证"
                disabled={controller.busy || (newAddress && !URL.canParse(address))}
                prepare={prepareQuickVerification}
              />
            </div>
          {:else}
            <p class="rounded-md bg-subtle p-4 text-sm text-fg-muted">
              提交后会生成验证配置，请按提示发布到站点。
            </p>
          {/if}

          {#if method !== 'BLOGSCLUB'}
            <button
              class={primaryClass}
              type="submit"
              disabled={controller.busy || controller.availability.checking}
            >
              {method === 'MANUAL' ? '提交人工审核' : '生成验证配置'}
            </button>
          {/if}
        {:else}
          <p class="rounded-md border border-line bg-subtle p-4 text-sm text-fg-muted">
            当前账号暂时不能对这个站点发起认证，请刷新站点信息后重试。
          </p>
        {/if}
      </section>
    {/if}

    <a class="{buttonClass} w-fit" href="/dashboard/sites/new">站点未收录，提交新增申请</a>
  </form>
{/if}

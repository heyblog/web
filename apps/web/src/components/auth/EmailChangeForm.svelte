<script lang="ts">
  import { onMount, untrack } from 'svelte';

  import {
    completeEmailChange,
    readEmailChange,
    updateEmailChange,
  } from '@/api/account-profile/email-change.browser';
  import type {
    EmailChangeSide,
    EmailChangeSite,
    EmailChangeStatus,
  } from '@/api/account-profile/email-change.types';
  import type { SessionUser } from '@/api/auth/auth.types';
  import { oauthProviders } from '@/api/auth/oauth.types';
  import { emailChangeMessage } from '@/application/auth/email-change.messages';

  import { buttonClass, inputClass, submitClass } from './email-change.styles';
  import EmailChangeVerification from './EmailChangeVerification.svelte';
  import OAuthButton from './OAuthButton.svelte';

  let {
    user,
    currentEmail,
    initial,
    initialError,
  }: {
    user: SessionUser;
    currentEmail: string;
    initial: EmailChangeStatus | null;
    initialError: string | null;
  } = $props();

  let status = $state(untrack(() => initial));
  let error = $state(
    untrack(() => initialError ?? (initial ? '' : emailChangeMessage('unavailable'))),
  );
  let notice = $state('');
  let ready = $state(false);
  let busy = $state(false);
  let uncertain = $state(false);
  let signIn = $state(false);
  let cancelConfirmation = $state(false);
  let email = $state('');
  let password = $state('');
  const dateFormatter = new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'medium',
    timeStyle: 'short',
    timeZone: 'Asia/Shanghai',
  });
  const request = $derived(status?.request);
  const canComplete = $derived(
    request?.old_verified &&
      request.new_verified &&
      status?.sites.every((site) => site.status === 'verified'),
  );
  const providers = $derived(
    oauthProviders.filter((provider) =>
      user.oauth_providers.some((bound) => bound.toLowerCase() === provider),
    ),
  );
  const disabled = $derived(!ready || busy || uncertain || signIn);
  const siteLabels: Readonly<Record<EmailChangeSite['status'], string>> = {
    required: '待复验',
    pending: '复验中',
    verified: '已通过',
    rejected: '未通过',
  };

  onMount(() => {
    ready = true;
  });

  function showError(code: string) {
    signIn = code === 'session_expired';
    error = emailChangeMessage(code);
  }

  async function reload() {
    const result = await readEmailChange();
    if (result.ok) {
      if (status?.request?.id !== result.value.request?.id) {
        cancelConfirmation = false;
        password = '';
      }
      status = result.value;
      uncertain = false;
    } else {
      showError(result.code === 'request_failed' ? 'unavailable' : result.code);
    }
  }

  async function refresh() {
    if (busy) return;
    busy = true;
    error = '';
    notice = '';
    try {
      await reload();
    } finally {
      busy = false;
    }
  }

  async function mutate(action: Parameters<typeof updateEmailChange>[0], message: string) {
    if (disabled) return false;
    busy = true;
    error = '';
    notice = '';
    try {
      const result = await updateEmailChange(action);
      if (result.ok) {
        status = result.value;
        notice = message;
        return true;
      }
      showError(result.code);
      if (result.uncertain && result.code !== 'mail_unavailable') {
        uncertain = true;
        error = '暂时无法确认操作结果，请先刷新换绑状态。';
      }
      if (
        ['email_change_active', 'email_change_missing', 'email_change_expired'].includes(
          result.code,
        )
      )
        await reload();
      return false;
    } finally {
      busy = false;
    }
  }

  async function start(event: SubmitEvent) {
    event.preventDefault();
    if (await mutate({ path: '', method: 'POST', body: { email: email.trim() } }, '申请已创建。'))
      email = '';
  }

  async function send(side: EmailChangeSide) {
    if (!request) return;
    await mutate(
      { path: '/send-code', method: 'POST', id: request.id, body: { side } },
      side === 'old' ? '验证码已发送至原邮箱。' : '验证码已发送至新邮箱。',
    );
  }

  async function verify(side: EmailChangeSide, code: string) {
    if (!request) return false;
    return mutate(
      { path: '/verify-code', method: 'POST', id: request.id, body: { side, code } },
      side === 'old' ? '已完成原邮箱验证。' : '已完成新邮箱验证。',
    );
  }

  async function verifyPassword(event: SubmitEvent) {
    event.preventDefault();
    if (request) {
      const currentPassword = password;
      password = '';
      await mutate(
        {
          path: '/verify-password',
          method: 'POST',
          id: request.id,
          body: { password: currentPassword },
        },
        '已确认账号身份。',
      );
    }
  }

  async function cancel() {
    if (!request) return;
    if (await mutate({ path: '', method: 'DELETE', id: request.id }, '已取消邮箱更换。'))
      cancelConfirmation = false;
  }

  async function complete() {
    if (!request || !canComplete || disabled) return;
    const requestedEmail = request.new_email;
    busy = true;
    error = '';
    notice = '';
    try {
      const result = await completeEmailChange(request.id);
      if (result.ok) {
        window.location.replace('/dashboard/profile?status=email-updated');
        return;
      }
      showError(result.code);
      if (result.uncertain) {
        uncertain = true;
        error = '暂时无法确认邮箱是否已更换。正在查询最新状态。';
        await reload();
        if (!uncertain && !status?.request) {
          window.location.replace('/dashboard/profile');
          return;
        }
        if (!signIn && uncertain) error = '暂时无法确认邮箱是否已更换，请刷新换绑状态。';
        if (!signIn && !uncertain && status?.request?.new_email === requestedEmail)
          error = '邮箱更换尚未完成，请重新提交。';
      } else if (
        [
          'email_change_sites_required',
          'email_change_missing',
          'email_change_expired',
          'email_change_verification_required',
        ].includes(result.code)
      ) {
        await reload();
      }
    } finally {
      busy = false;
    }
  }

  function proofHref(site: EmailChangeSite, id: string) {
    if (site.status === 'pending' || site.status === 'verified')
      return `/dashboard/claims/${encodeURIComponent(site.claim_id)}`;
    const search = new URLSearchParams({ short_id: site.short_id, email_change_id: id });
    return `/dashboard/claims?${search}`;
  }
</script>

<div class="grid gap-5" aria-busy={busy}>
  <p class="text-sm break-all text-fg-muted">
    当前邮箱 <span class="font-medium text-fg">{currentEmail}</span>
  </p>

  {#if error}
    <div class="rounded-md bg-danger-bg p-3 text-sm text-danger-fg" role="alert">
      <p>{error}</p>
      {#if signIn}
        <a
          class="inline-flex min-h-11 items-center underline sm:min-h-10"
          href="/login?next=%2Fdashboard%2Fprofile">重新登录</a
        >
      {/if}
    </div>
  {/if}
  {#if notice}<p class="text-sm text-success-fg" role="status">{notice}</p>{/if}
  {#if busy}<p class="text-sm text-fg-muted" role="status">正在处理…</p>{/if}

  {#if status?.sites.length}
    <section class="grid gap-2" aria-labelledby="email-change-sites-heading">
      <h3 id="email-change-sites-heading" class="text-sm font-semibold">需要复验的站点</h3>
      <p class="text-sm text-fg-muted">
        全部站点复验通过后才能更换邮箱。等待期间，已有认证和管理权限继续有效。
      </p>
      <ul class="divide-y divide-line rounded-md border border-line px-3">
        {#each status.sites as site (site.short_id)}
          <li class="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
            <div class="grid min-w-0 gap-1">
              <span class="text-sm font-medium wrap-anywhere">{site.name}</span>
              <span class="text-xs break-all text-fg-muted">{site.address}</span>
              {#if request}<span class="text-xs text-fg-muted">{siteLabels[site.status]}</span>{/if}
            </div>
            {#if request}
              <a
                class={`${buttonClass} w-fit`}
                href={proofHref(site, request.id)}
                aria-label={`${site.status === 'required' || site.status === 'rejected' ? '复验' : '查看复验'} ${site.name}`}
                data-astro-prefetch="false"
              >
                {site.status === 'required' || site.status === 'rejected' ? '开始复验' : '查看复验'}
              </a>
            {/if}
          </li>
        {/each}
      </ul>
    </section>
  {/if}

  {#if status && !request}
    <form method="post" onsubmit={start}>
      <fieldset class="grid min-w-0 gap-2" {disabled}>
        <label class="text-sm font-medium" for="new-account-email">新邮箱</label>
        <input
          id="new-account-email"
          class={inputClass}
          type="email"
          name="email"
          autocomplete="email"
          maxlength="320"
          bind:value={email}
          required
        />
        <button type="submit" class={`${submitClass} w-fit`}>申请更换邮箱</button>
      </fieldset>
    </form>
  {:else if request}
    {#key request.id}
      <p class="text-sm text-fg-muted">
        申请有效期至 <time datetime={request.expires_at}
          >{dateFormatter.format(new Date(request.expires_at))}（北京时间）</time
        >
      </p>
      <EmailChangeVerification
        side="old"
        email={currentEmail}
        verified={request.old_verified}
        {disabled}
        onSend={send}
        onVerify={verify}
      />
      {#if !request.old_verified}
        <details class="grid rounded-md border border-line px-3">
          <summary class="min-h-11 cursor-pointer py-3 text-sm font-medium sm:min-h-10"
            >无法使用原邮箱</summary
          >
          <div class="grid gap-4 pb-3">
            {#if user.has_password}
              <form method="post" onsubmit={verifyPassword}>
                <fieldset class="grid min-w-0 gap-2" {disabled}>
                  <label class="text-sm font-medium" for="email-change-password">当前密码</label>
                  <input
                    id="email-change-password"
                    class={inputClass}
                    type="password"
                    name="password"
                    autocomplete="current-password"
                    bind:value={password}
                    required
                  />
                  <button type="submit" class={`${buttonClass} w-fit`}>用密码确认身份</button>
                </fieldset>
              </form>
            {/if}
            {#each providers as provider (provider)}
              <OAuthButton
                {provider}
                intent="email-change"
                emailChangeId={request.id}
                next="/dashboard/profile"
                label={`用已绑定的 ${provider === 'github' ? 'GitHub' : 'BlogsClub'} 确认身份`}
                {disabled}
                oncomplete={() => void refresh()}
              />
            {/each}
            {#if !user.has_password && providers.length === 0}
              <p class="text-sm text-fg-muted">当前账号需要通过原邮箱验证码确认身份。</p>
            {/if}
          </div>
        </details>
      {/if}
      <EmailChangeVerification
        side="new"
        email={request.new_email}
        verified={request.new_verified}
        {disabled}
        onSend={send}
        onVerify={verify}
      />
      {#if canComplete}
        <p class="text-sm text-fg-muted">确认更换后，其他设备需要重新登录。</p>
        <button
          type="button"
          class={`${submitClass} w-fit`}
          {disabled}
          onclick={() => void complete()}>确认更换邮箱</button
        >
      {/if}
      {#if cancelConfirmation}
        <div class="grid gap-2 rounded-md bg-subtle p-3">
          <p class="text-sm">取消后，这次申请的邮箱验证和站点复验进度会失效。</p>
          <div class="flex flex-wrap gap-2">
            <button type="button" class={buttonClass} {disabled} onclick={() => void cancel()}
              >确认取消申请</button
            >
            <button
              type="button"
              class={buttonClass}
              {disabled}
              onclick={() => (cancelConfirmation = false)}>继续换绑</button
            >
          </div>
        </div>
      {:else}
        <button
          type="button"
          class={`${buttonClass} w-fit`}
          {disabled}
          onclick={() => (cancelConfirmation = true)}>取消换绑</button
        >
      {/if}
    {/key}
  {/if}

  {#if !signIn && (request || uncertain || !status)}
    <button
      type="button"
      class={`${buttonClass} w-fit`}
      disabled={!ready || busy}
      onclick={() => void refresh()}
    >
      刷新换绑状态
    </button>
  {/if}
</div>

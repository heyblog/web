<script lang="ts">
  import { untrack } from 'svelte';

  import { mutateManagedUser } from '@/api/managed-users/managed-users.browser';
  import type {
    AccountOperation,
    ManagedUser,
    ManagedUserMutation,
  } from '@/api/managed-users/managed-users.types';
  import { formFeedback, showFieldIssue } from '@/application/forms/field-feedback.browser';
  import { authorizationError } from '@/application/management/users.messages';
  import {
    accountManagementReason,
    userMutationNeedsReload,
  } from '@/application/management/users.model';

  import { buttonClass, inputClass, panelClass, primaryClass } from './users.styles';
  let {
    user,
    actor,
    disabled,
    onsaved,
    onbusy,
    ondirty,
    onuncertain,
  }: {
    user: ManagedUser;
    actor: ManagedUser;
    disabled: boolean;
    onsaved: (user: ManagedUser) => void;
    onbusy: (value: boolean) => void;
    ondirty: (value: boolean) => void;
    onuncertain: (value: boolean) => void;
  } = $props();
  let displayName = $state(untrack(() => user.display_name));
  let email = $state(untrack(() => user.email ?? ''));
  let busy = $state(false);
  let error = $state('');
  let formElement: HTMLFormElement;
  let notice = $state('');
  let reloadRequired = $state(false);
  let confirmation = $state<AccountOperation | null>(null);
  const dirty = $derived(
    displayName.trim() !== user.display_name || email.trim() !== (user.email ?? ''),
  );
  const reason = $derived(accountManagementReason(actor, user));
  const locked = $derived(disabled || busy || reloadRequired || !!reason);
  $effect(() => onbusy(busy));
  $effect(() => onuncertain(reloadRequired));
  $effect(() => ondirty(dirty));
  async function save(operation: AccountOperation, confirmed = false): Promise<void> {
    if (locked) return;
    if (
      !confirmed &&
      (operation === 'status' ||
        operation === 'sessions/revoke' ||
        (operation === 'profile' && email.trim() !== (user.email ?? '')))
    ) {
      confirmation = operation;
      return;
    }
    if (operation !== 'profile' && dirty) return;
    busy = true;
    error = '';
    notice = '';
    confirmation = null;
    const base = { updated_at: user.updated_at };
    const change: ManagedUserMutation =
      operation === 'profile'
        ? { operation, ...base, display_name: displayName.trim(), email: email.trim() }
        : operation === 'status'
          ? { operation, ...base, status: user.access_status === 'ACTIVE' ? 'SUSPENDED' : 'ACTIVE' }
          : { operation, ...base };
    const result = await mutateManagedUser(user.id, change);
    busy = false;
    if (!result.ok) {
      const field =
        result.code === 'invalid_username' || result.code === 'username_taken'
          ? 'username'
          : result.code === 'invalid_display_name'
            ? 'display_name'
            : result.code === 'invalid_email' || result.code === 'email_taken'
              ? 'email'
              : '';
      const message = authorizationError(result.code, result.status);
      if (!field || !showFieldIssue(formElement, field, message)) error = message;
      reloadRequired =
        userMutationNeedsReload(result.code, result.status) ||
        [401, 403, 404].includes(result.status);
      return;
    }
    displayName = result.user.display_name;
    email = result.user.email ?? '';
    onsaved(result.user);
    notice =
      result.mailSent === false
        ? '邮件发送失败，请稍后重新发送。'
        : result.mailSent === true
          ? '邮件已发送，请用户查收。'
          : operation === 'sessions/revoke'
            ? '该用户的登录会话已撤销。'
            : '账号资料已更新。';
  }
</script>

<section class={`${panelClass} mb-6`} aria-labelledby="account-heading" aria-busy={busy}>
  <h2 id="account-heading" class="text-lg font-semibold">账号资料与登录</h2>
  <dl class="my-5 grid gap-4 text-sm sm:grid-cols-2">
    <div>
      <dt class="text-fg-muted">注册时间</dt>
      <dd class="mt-1">{new Date(user.created_at).toLocaleString('zh-CN')}</dd>
    </div>
    <div>
      <dt class="text-fg-muted">最近登录</dt>
      <dd class="mt-1">
        {user.last_login_at ? new Date(user.last_login_at).toLocaleString('zh-CN') : '尚未登录'}
      </dd>
    </div>
    <div>
      <dt class="text-fg-muted">密码登录</dt>
      <dd class="mt-1">{user.has_password ? '已设置密码' : '尚未设置密码'}</dd>
    </div>
    <div>
      <dt class="text-fg-muted">第三方登录</dt>
      <dd class="mt-1">
        {user.oauth_providers
          .map((provider) => (provider === 'GITHUB' ? 'GitHub' : 'BlogsClub'))
          .join('、') || '尚未绑定'}
      </dd>
    </div>
  </dl>
  {#if reason}<p class="mb-4 text-sm text-fg-muted">{reason}</p>{/if}
  <form
    bind:this={formElement}
    use:formFeedback
    class="grid gap-4"
    onsubmit={(event) => {
      event.preventDefault();
      void save('profile');
    }}
  >
    <fieldset class="grid gap-4 sm:grid-cols-2" disabled={locked}>
      <label class="grid gap-2 text-sm font-medium"
        >显示名称<input
          class={inputClass}
          bind:value={displayName}
          required
          autocomplete="off"
          name="display_name"
          data-validation="display-name"
          aria-describedby="field-display_name-help"
        /><span id="field-display_name-help" class="text-xs/normal font-normal text-fg-muted"
          >1–128 个字符，不能只填写空格。</span
        ></label
      >
      <label class="grid gap-2 text-sm font-medium"
        >邮箱<input
          class={inputClass}
          bind:value={email}
          type="email"
          required
          maxlength="320"
          autocomplete="off"
          name="email"
        /></label
      >
    </fieldset>
    <p class="text-xs text-fg-muted">更换邮箱后，用户需要验证新邮箱并重新登录。</p>
    <div class="flex flex-wrap gap-3">
      <button class={dirty ? primaryClass : buttonClass} type="submit" disabled={locked || !dirty}
        >保存资料</button
      >{#if dirty}<button
          class={buttonClass}
          type="button"
          disabled={busy}
          onclick={() => {
            displayName = user.display_name;
            email = user.email ?? '';
            confirmation = null;
          }}>撤销修改</button
        >{/if}
    </div>
  </form>
  {#if confirmation}<div class="mt-5 grid gap-3 rounded-md bg-warning-bg p-4 text-sm" role="alert">
      <p>
        {confirmation === 'profile'
          ? '确认更换邮箱？当前登录会话会失效，用户需要验证新邮箱。'
          : confirmation === 'sessions/revoke'
            ? '确认撤销该用户的全部登录会话？用户需要重新登录。'
            : user.access_status === 'ACTIVE'
              ? '确认停用账号？该用户将无法登录，已有会话立即失效。'
              : '确认恢复此账号的登录权限？'}
      </p>
      <div class="flex gap-3">
        <button
          class={buttonClass}
          type="button"
          disabled={busy}
          onclick={() => (confirmation = null)}>取消</button
        ><button
          class={primaryClass}
          type="button"
          disabled={busy}
          onclick={() => confirmation && save(confirmation, true)}>确认操作</button
        >
      </div>
    </div>{/if}
  <div class="mt-6 flex flex-wrap gap-3 border-t border-line pt-5">
    <button
      class={buttonClass}
      type="button"
      disabled={locked || dirty}
      onclick={() => save('status')}
      >{user.access_status === 'ACTIVE' ? '停用账号' : '恢复账号'}</button
    >
    <button
      class={buttonClass}
      type="button"
      disabled={locked || dirty}
      onclick={() => save('sessions/revoke')}>撤销登录会话</button
    >
    <button
      class={buttonClass}
      type="button"
      disabled={locked || dirty || !user.email || user.access_status !== 'ACTIVE'}
      onclick={() => save('password-reset')}
      >{user.has_password ? '发送密码重置邮件' : '发送密码设置邮件'}</button
    >
    <button
      class={buttonClass}
      type="button"
      disabled={locked ||
        dirty ||
        !user.email ||
        user.email_verified ||
        user.access_status !== 'ACTIVE'}
      onclick={() => save('verification')}>发送验证邮件</button
    >
  </div>
  {#if disabled}<p class="mt-3 text-xs text-fg-muted">请先完成当前授权修改。</p>{/if}
  {#if error}<p class="mt-4 rounded-md bg-danger-bg p-3 text-sm text-danger-fg" role="alert">
      {error}
    </p>{/if}
  {#if reloadRequired}<a
      class="mt-3 inline-flex min-h-11 items-center text-sm text-tint-fg underline"
      href={`/management/users/${user.id}`}>重新加载用户资料</a
    >{/if}
  {#if notice}<p class="mt-4 text-sm text-success-fg" role="status">{notice}</p>{/if}
</section>

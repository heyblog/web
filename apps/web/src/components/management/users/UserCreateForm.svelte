<script lang="ts">
  import { onMount } from 'svelte';

  import { mutateManagedUser } from '@/api/managed-users/managed-users.browser';
  import type { ManagedUser } from '@/api/managed-users/managed-users.types';
  import { formFeedback, showFieldIssue } from '@/application/forms/field-feedback.browser';
  import { authorizationError } from '@/application/management/users.messages';
  import { userMutationNeedsReload } from '@/application/management/users.model';
  import { protectFormDraft } from '@/application/navigation/form-navigation.browser';

  import { buttonClass, inputClass, panelClass, primaryClass } from './users.styles';
  let username = $state('');
  let displayName = $state('');
  let email = $state('');
  let pending = $state(false);
  let error = $state('');
  let formElement: HTMLFormElement;
  let created = $state<ManagedUser | null>(null);
  let mailSent = $state(false);
  let uncertain = $state(false);
  onMount(() =>
    protectFormDraft(() => ({
      dirty: !created && !!(username || displayName || email),
      busy: pending,
    })),
  );
  async function submit(): Promise<void> {
    if (pending || uncertain) return;
    pending = true;
    error = '';
    const result = await mutateManagedUser(
      created?.id ?? null,
      created
        ? { operation: 'password-reset', updated_at: created.updated_at }
        : {
            operation: 'create',
            username: username.trim().toLowerCase(),
            display_name: displayName.trim(),
            email: email.trim(),
          },
    );
    pending = false;
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
      uncertain = userMutationNeedsReload(result.code, result.status);
      return;
    }
    created = result.user;
    mailSent = result.mailSent === true;
  }
</script>

<section class={panelClass} aria-busy={pending}>
  {#if created}
    <h2 class="text-lg font-semibold">账号已创建</h2>
    <p class="mt-3 text-sm" role="status">{created.display_name}（@{created.username}）</p>
    <p class="my-4 text-sm text-fg-muted">
      {mailSent
        ? '密码设置邮件已发送，用户可通过邮件完成设置。'
        : '密码设置邮件发送失败，可重新发送。'}
    </p>
    <div class="flex flex-wrap gap-3">
      <a class={buttonClass} href={`/management/users/${created.id}`}>查看用户资料</a
      >{#if !mailSent}<button
          class={primaryClass}
          type="button"
          disabled={pending || uncertain}
          onclick={submit}>重新发送设置邮件</button
        >{/if}
    </div>
  {:else}
    <p class="mb-5 text-sm text-fg-muted">新账号为普通用户，创建后将向邮箱发送密码设置链接。</p>
    <form
      bind:this={formElement}
      use:formFeedback
      class="grid max-w-lg gap-5"
      onsubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      <fieldset class="grid gap-5" disabled={pending || uncertain}>
        <label class="grid gap-2 text-sm font-medium"
          >用户名<input
            class={inputClass}
            bind:value={username}
            pattern={'[a-z0-9_]{3,32}'}
            minlength="3"
            maxlength="32"
            required
            autocomplete="off"
            name="username"
            aria-describedby="create-username-help"
            data-validation="username"
            autocapitalize="none"
            spellcheck="false"
          /><span id="create-username-help" class="text-xs text-fg-muted"
            >3–32 位小写字母、数字或下划线。</span
          ></label
        >
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
            maxlength="320"
            required
            autocomplete="off"
            name="email"
          /></label
        >
      </fieldset>
      <button class={primaryClass} type="submit" disabled={pending || uncertain}
        >{pending ? '正在创建…' : '创建用户并发送邮件'}</button
      >
    </form>
  {/if}
  {#if error}<p class="mt-4 text-sm text-danger-fg" role="alert">{error}</p>{/if}
  {#if uncertain}<a
      class={`${buttonClass} mt-4`}
      href={`/management/users?q=${encodeURIComponent(email)}`}>在用户列表核实结果</a
    >{/if}
</section>

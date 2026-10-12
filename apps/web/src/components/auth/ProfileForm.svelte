<script lang="ts">
  import { tick, untrack } from 'svelte';

  import type { ProfileAction } from '../../api/account-profile/profile.types';
  import type { SessionUser } from '../../api/auth/auth.types';
  import { createProfileController } from '../../application/auth/profile.controller.svelte';

  let {
    user,
    kind,
    next = '/dashboard',
  }: { user: SessionUser; kind: ProfileAction['kind']; next?: string } = $props();
  const form = createProfileController(untrack(() => user));
  let confirmationHeading: HTMLHeadingElement | undefined = $state();
  let usernameInput: HTMLInputElement | undefined = $state();
  const inputClass =
    'min-h-11 w-full rounded-md border border-line-strong bg-surface px-3 text-base outline-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:cursor-not-allowed disabled:bg-subtle disabled:text-fg-subtle sm:min-h-10';
  const buttonClass =
    'min-h-11 rounded-md px-4 text-sm font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:cursor-not-allowed disabled:opacity-50 sm:min-h-10';
  const dirty = $derived(
    kind === 'display'
      ? form.displayName !== form.user.display_name
      : form.username !== form.user.username ||
          (kind === 'confirm' && form.displayName !== form.user.display_name),
  );

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    const saved = await form.submit(kind);
    if (saved && kind === 'confirm') window.location.assign(next);
    else if (saved && kind === 'username') window.location.reload();
    else if (form.confirming) {
      await tick();
      confirmationHeading?.focus();
    }
  }

  function beforeUnload(event: BeforeUnloadEvent) {
    if (dirty || form.busy) event.preventDefault();
  }
</script>

<svelte:window onbeforeunload={beforeUnload} />

<form class="grid gap-5" onsubmit={submit} aria-busy={form.busy}>
  {#if form.error}
    <div class="rounded-md bg-danger-bg p-3 text-sm/normal text-danger-fg" role="alert">
      <p>{form.error}</p>
      {#if form.uncertain}<a
          class="mt-2 inline-flex min-h-11 items-center underline sm:min-h-10"
          href={kind === 'confirm'
            ? `/complete-profile?next=${encodeURIComponent(next)}`
            : '/dashboard/profile'}>刷新资料</a
        >{/if}
      {#if form.signIn}<a
          class="mt-2 inline-flex min-h-11 items-center underline sm:min-h-10"
          href={`/login?next=${encodeURIComponent(kind === 'confirm' ? '/complete-profile' : '/dashboard/profile')}`}
          >重新登录</a
        >{/if}
    </div>
  {/if}
  {#if form.status}<p class="text-sm text-success-fg" role="status">{form.status}</p>{/if}
  <fieldset class="grid min-w-0 gap-5" disabled={form.busy || form.uncertain}>
    {#if kind !== 'display'}
      <div class="grid gap-1.5">
        <label class="text-sm font-medium" for={`${kind}-username`}>用户名</label>
        <input
          bind:this={usernameInput}
          bind:value={form.username}
          class={inputClass}
          id={`${kind}-username`}
          name="username"
          autocomplete="username"
          autocapitalize="none"
          spellcheck="false"
          pattern={'[a-z0-9_]{3,32}'}
          minlength="3"
          maxlength="32"
          required
          readonly={form.confirming ||
            (kind === 'username' && form.user.username_changes_remaining === 0)}
          aria-describedby={`${kind}-username-help`}
        />
        <p class="text-xs/normal text-fg-muted" id={`${kind}-username-help`}>
          {kind === 'confirm'
            ? '3–32 位小写字母、数字或下划线。确认后还可修改一次。'
            : `剩余修改次数：${form.user.username_changes_remaining}。3–32 位小写字母、数字或下划线。`}
        </p>
      </div>
    {/if}
    {#if kind !== 'username'}
      <div class="grid gap-1.5">
        <label class="text-sm font-medium" for={`${kind}-display`}>显示名称</label>
        <input
          bind:value={form.displayName}
          class={inputClass}
          id={`${kind}-display`}
          name="display_name"
          autocomplete="nickname"
          required
          aria-describedby={`${kind}-display-help`}
        />
        <p class="text-xs/normal text-fg-muted" id={`${kind}-display-help`}>
          1–128 个字符，可随时修改。
        </p>
      </div>
    {/if}
    {#if form.confirming}
      <section class="grid gap-3 rounded-md bg-subtle p-4" aria-labelledby="username-confirmation">
        <h3
          bind:this={confirmationHeading}
          id="username-confirmation"
          class="text-sm font-semibold outline-none"
          tabindex="-1"
        >
          确认修改为 {form.username}？
        </h3>
        <p class="text-sm/normal text-fg-muted">
          保存后将用完用户名修改次数，其他设备需要重新登录。
        </p>
        <div class="flex flex-wrap gap-3">
          <button
            type="submit"
            class={`${buttonClass} border border-line-strong bg-surface hover:bg-subtle`}
            >{form.busy ? '保存中…' : '确认修改'}</button
          >
          <button
            type="button"
            class={`${buttonClass} hover:bg-surface`}
            onclick={() => {
              form.cancelConfirmation();
              usernameInput?.focus();
            }}>返回编辑</button
          >
        </div>
      </section>
    {:else if kind !== 'username' || form.user.username_changes_remaining > 0}
      <button
        class={`${buttonClass} ${kind === 'username' ? 'border border-line-strong hover:bg-subtle' : 'bg-primary text-primary-fg hover:bg-primary-hover'}`}
        type="submit"
        disabled={kind !== 'confirm' && !dirty}
      >
        {form.busy
          ? '保存中…'
          : kind === 'confirm'
            ? '确认并继续'
            : kind === 'display'
              ? '保存显示名称'
              : '修改用户名'}
      </button>
    {/if}
  </fieldset>
</form>

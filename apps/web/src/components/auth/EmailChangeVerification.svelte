<script lang="ts">
  import type { EmailChangeSide } from '@/api/account-profile/email-change.types';

  import { buttonClass, inputClass } from './email-change.styles';

  let {
    side,
    email,
    verified,
    disabled,
    onSend,
    onVerify,
  }: {
    side: EmailChangeSide;
    email: string;
    verified: boolean;
    disabled: boolean;
    onSend: (side: EmailChangeSide) => Promise<void>;
    onVerify: (side: EmailChangeSide, code: string) => Promise<boolean>;
  } = $props();

  let code = $state('');

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (await onVerify(side, code)) code = '';
  }
</script>

<div class="grid gap-2">
  <div>
    <h3 class="text-sm font-semibold">{side === 'old' ? '原邮箱' : '新邮箱'}</h3>
    <p class="text-sm break-all text-fg-muted">{email}</p>
  </div>
  {#if verified}
    <p class="text-sm text-success-fg">已验证</p>
  {:else}
    <form method="post" onsubmit={submit}>
      <fieldset class="grid min-w-0 gap-2" {disabled}>
        <label class="text-sm font-medium" for={`email-change-code-${side}`}>验证码</label>
        <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
          <input
            id={`email-change-code-${side}`}
            class={inputClass}
            type="text"
            name="code"
            inputmode="numeric"
            autocomplete="one-time-code"
            pattern={'[0-9]{6}'}
            maxlength="6"
            bind:value={code}
            required
          />
          <button type="button" class={buttonClass} onclick={() => void onSend(side)}>
            发送验证码
          </button>
        </div>
        <button type="submit" class={`${buttonClass} w-fit`} disabled={!/^[0-9]{6}$/u.test(code)}>
          {side === 'old' ? '验证原邮箱' : '验证新邮箱'}
        </button>
      </fieldset>
    </form>
  {/if}
</div>

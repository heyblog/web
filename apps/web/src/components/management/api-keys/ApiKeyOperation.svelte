<script lang="ts">
  import { untrack } from 'svelte';

  import { defaultExpiry, expirationPayload } from '@/application/api-keys/api-keys.model';
  import type { ApiClientSummary, ApiKeyIssuePayload } from '@/application/api-keys/api-keys.types';

  import {
    buttonClass,
    dangerClass,
    errorClass,
    inputClass,
    primaryClass,
  } from './api-keys.styles';
  import ApiKeyExpiryFields from './ApiKeyExpiryFields.svelte';
  interface Props {
    kind: 'rotate' | 'issue' | 'toggle' | 'revoke';
    client: ApiClientSummary;
    busy: boolean;
    prefix?: string;
    onconfirm: (value: { overlap: number; expiration: ApiKeyIssuePayload | null }) => Promise<void>;
    oncancel: () => void;
    ondirty: () => void;
  }
  let { kind, client, busy, prefix, onconfirm, oncancel, ondirty }: Props = $props();
  let overlap = $state(0);
  let expiresAt = $state(untrack(() => defaultExpiry(client.audience)));
  let neverExpires = $state(false);
  let error = $state<string | null>(null);
  let destructive = $derived(
    kind === 'revoke' || (kind === 'toggle' && client.disabled_at === null),
  );
  let label = $derived(
    kind === 'rotate'
      ? '确认轮换'
      : kind === 'issue'
        ? '签发新密钥'
        : kind === 'revoke'
          ? '确认撤销'
          : client.disabled_at
            ? '确认启用'
            : '确认停用',
  );

  async function submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    error = null;
    let expiration: ApiKeyIssuePayload | null = null;
    if (kind === 'issue') {
      const date = new Date(expiresAt);
      expiration = expirationPayload(
        client.audience,
        {
          expires_at: neverExpires || !Number.isFinite(date.getTime()) ? null : date.toISOString(),
          never_expires: neverExpires,
        },
        Date.now(),
      );
      if (expiration === null) {
        error =
          client.audience === 'INTERNAL'
            ? '到期时间必须在未来 90 天内。'
            : '请选择未来的到期时间。';
        return;
      }
    }
    await onconfirm({ overlap, expiration });
  }
</script>

<form class="flex flex-1 flex-col" onsubmit={submit} oninput={ondirty} onchange={ondirty}>
  <fieldset class="grid gap-5 p-6" disabled={busy}>
    <p class="font-medium wrap-break-word">{client.name}</p>
    {#if kind === 'rotate'}
      <p class="text-sm text-fg-muted">
        生成新密钥，并让最近一把有效密钥在所选重叠时间后失效。请及时更新调用服务。
      </p>
      <label class="grid gap-2 text-sm font-medium"
        >旧密钥重叠时间<select class={inputClass} bind:value={overlap}
          ><option value={0}>立即失效</option><option value={1}>1 小时</option><option value={6}
            >6 小时</option
          ><option value={24}>24 小时</option></select
        ></label
      >
    {:else if kind === 'issue'}
      <ApiKeyExpiryFields audience={client.audience} bind:expiresAt bind:neverExpires />
    {:else if kind === 'revoke'}
      <code class="text-sm break-all">{prefix}…</code>
      <p class="text-sm text-fg-muted">
        撤销后，此密钥立即失效且无法恢复。同一调用方的其他密钥不受影响。
      </p>
    {:else}
      <p class="text-sm text-fg-muted">
        {client.disabled_at
          ? '启用后，尚未过期且未撤销的密钥将恢复调用能力。'
          : '停用后，此调用方的所有密钥都无法调用接口。之后可以重新启用。'}
      </p>
    {/if}
    {#if error}<p class={errorClass} role="alert">{error}</p>{/if}
  </fieldset>
  <footer
    class="sticky bottom-0 mt-auto flex justify-end gap-3 border-t border-line bg-surface px-6 py-4"
  >
    <button class={buttonClass} type="button" disabled={busy} onclick={oncancel}>取消</button
    ><button class={destructive ? dangerClass : primaryClass} disabled={busy} aria-busy={busy}
      >{busy ? '正在处理' : label}</button
    >
  </footer>
</form>

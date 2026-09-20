<script lang="ts">
  import type { ApiClientAudience } from '@/application/api-keys/api-keys.types';

  import { inputClass } from './api-keys.styles';
  interface Props {
    audience: ApiClientAudience;
    expiresAt: string;
    neverExpires: boolean;
  }
  let { audience, expiresAt = $bindable(), neverExpires = $bindable() }: Props = $props();
</script>

<label class="grid gap-2 text-sm font-medium">
  到期时间
  <input
    class={inputClass}
    name="expires_at"
    type="datetime-local"
    bind:value={expiresAt}
    disabled={neverExpires}
    required={!neverExpires}
  />
  <span class="text-xs font-normal text-fg-muted"
    >{audience === 'INTERNAL'
      ? '内部密钥最长有效 90 天。'
      : '外部密钥默认有效 365 天。'}时间使用当前设备时区。</span
  >
</label>
{#if audience === 'EXTERNAL'}
  <label class="flex min-h-11 items-center gap-2 text-sm sm:min-h-10">
    <input class="size-4 accent-primary" type="checkbox" bind:checked={neverExpires} />永不过期
  </label>
  {#if neverExpires}<p class="text-xs text-fg-muted">请定期轮换长期使用的密钥。</p>{/if}
{/if}

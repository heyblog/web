<script lang="ts">
  import { IconCheck, IconCopy } from '@tabler/icons-svelte';

  import type { ApiCredential } from '@/application/api-keys/api-keys.types';

  import { buttonClass, errorClass, primaryClass } from './api-keys.styles';
  interface Props {
    credential: ApiCredential;
    onclose: () => void;
  }
  let { credential, onclose }: Props = $props();
  let copied = $state(false);
  let error = $state<string | null>(null);
  async function copy(): Promise<void> {
    error = null;
    try {
      await navigator.clipboard.writeText(credential.token);
      copied = true;
    } catch (cause) {
      if (!(cause instanceof Error)) throw cause;
      error = '无法自动复制，请选中下方密钥并手动复制。';
    }
  }
</script>

<div class="grid gap-5 p-6">
  <p class="font-medium wrap-break-word">{credential.client.name}</p>
  <p class="text-sm text-fg-muted">此密钥仅显示一次。关闭前请保存到服务端密钥管理系统。</p>
  <code
    class="block rounded-md border border-line bg-subtle p-4 font-mono text-sm break-all select-all"
    aria-label="新生成的 API 密钥">{credential.token}</code
  >
  {#if error}<p class={errorClass} role="alert">{error}</p>{/if}
  <p class="sr-only" role="status">{copied ? '密钥已复制' : ''}</p>
  <div class="flex flex-wrap justify-end gap-3">
    <button class={buttonClass} onclick={copy}
      >{#if copied}<IconCheck size={18} aria-hidden="true" />{:else}<IconCopy
          size={18}
          aria-hidden="true"
        />{/if}{copied ? '已复制' : '复制密钥'}</button
    ><button class={primaryClass} onclick={onclose}>完成</button>
  </div>
</div>

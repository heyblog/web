<script lang="ts">
  import { onDestroy } from 'svelte';

  import { ClaimProblem, createClaimsTransport } from '@/api/site-management/claims.browser';
  import type { ClaimAttachment } from '@/api/site-management/claims.types';
  import { claimMessage } from '@/application/site-management/claims.model';
  import { buttonClass } from '@/components/management/api-keys/api-keys.styles';
  let {
    attachments,
    management = false,
  }: { attachments: readonly ClaimAttachment[]; management?: boolean } = $props();
  const transport = createClaimsTransport();
  let busy = $state(false);
  let error = $state('');
  let operation: AbortController | null = null;
  onDestroy(() => operation?.abort());
  async function download(id: string) {
    if (busy) return;
    busy = true;
    error = '';
    operation = new AbortController();
    try {
      const url = await transport.downloadAttachment(id, operation.signal, management);
      if (!operation.signal.aborted) window.location.assign(url);
    } catch (failure) {
      if (operation.signal.aborted) return;
      if (failure instanceof ClaimProblem) error = claimMessage(failure.code);
      else throw failure;
    } finally {
      busy = false;
    }
  }
</script>

{#if attachments.length}
  <section class="grid gap-2" aria-label="证明附件">
    <h3 class="text-sm font-medium">证明附件</h3>
    <ul class="grid gap-2">
      {#each attachments as attachment (attachment.id)}
        <li class="grid gap-2 rounded-md border border-line bg-subtle p-3 text-sm">
          <span class="font-medium break-all">{attachment.filename}</span>
          <span class="text-xs text-fg-muted"
            >{(attachment.size / (1024 * 1024)).toFixed(2)} MiB</span
          >
          {#if attachment.status === 'BOUND' || attachment.status === 'ARCHIVED'}
            <button
              class={buttonClass}
              type="button"
              disabled={busy}
              onclick={() => void download(attachment.id)}
              aria-label={`下载 ${attachment.filename}`}>下载附件</button
            >
          {:else}<span class="text-xs text-fg-muted">附件已不可用</span>{/if}
        </li>
      {/each}
    </ul>
    {#if error}<p class="text-sm text-danger-fg" role="alert">{error}</p>{/if}
  </section>
{/if}

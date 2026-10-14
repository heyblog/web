<script lang="ts">
  import { tick } from 'svelte';

  import type { SiteAvailabilityController } from '@/application/site-submission/site-availability.controller.svelte';
  import { siteAvailabilityTarget } from '@/application/site-submission/site-submission.api.browser';
  import InlineAlert from '@/components/feedback/InlineAlert.svelte';

  interface Props {
    id: string;
    availability: SiteAvailabilityController;
    purpose: 'CREATE' | 'UPDATE' | 'VERIFY' | 'CORRECT' | 'APPROVE';
    onretry: () => Promise<boolean>;
    onedit?: () => void;
    correctionPath?: string;
  }
  let { id, availability, purpose, onretry, onedit, correctionPath }: Props = $props();
  const actionClass =
    'inline-flex min-h-11 w-fit items-center font-semibold underline underline-offset-4 sm:min-h-10';

  async function retry(): Promise<void> {
    await onretry();
    await tick();
    document.getElementById(id)?.focus();
  }
</script>

{#if availability.state !== 'idle'}
  <div {id} tabindex="-1" aria-busy={availability.checking}>
    {#if availability.state === 'checking'}
      <p class="text-sm text-fg-muted" role="status">正在检查站点地址…</p>
    {:else if availability.state === 'available'}
      <p class="text-sm text-fg-muted" role="status">该主页地址可以使用。</p>
    {:else if availability.state === 'duplicate' && availability.existingSite}
      {@const existing = availability.existingSite}
      <InlineAlert tone="warning">
        <p>
          {purpose === 'CREATE'
            ? existing.visibility === 'REMOVED'
              ? '该主页地址对应的站点已移除，请核对地址或申请恢复。'
              : '该主页地址已在目录中，请核对地址或填写更新申请。'
            : '该主页地址已被其他站点使用，请核对并修改地址。'}
        </p>
        <p class="mt-2 font-medium">{existing.name}</p>
        <p class="break-all">{existing.url}</p>
        {#snippet actions()}
          {#if onedit}<button class={actionClass} type="button" onclick={onedit}>修改地址</button
            >{/if}
          <button class={actionClass} type="button" onclick={retry}>重新检查</button>
          {#if purpose === 'CREATE'}
            <a class={actionClass} href={siteAvailabilityTarget(existing)}
              >{existing.visibility === 'REMOVED' ? '申请恢复站点' : '填写更新申请'}</a
            >
          {/if}
          {#if correctionPath}<a class={actionClass} href={correctionPath}>进入修正</a>{/if}
        {/snippet}
      </InlineAlert>
    {:else if availability.state === 'failed' || availability.state === 'invalid'}
      <InlineAlert tone="danger">
        <p>{availability.message}</p>
        {#snippet actions()}
          {#if onedit}<button class={actionClass} type="button" onclick={onedit}>修改地址</button
            >{/if}
          <button class={actionClass} type="button" onclick={retry}>重新检查</button>
          {#if correctionPath}<a class={actionClass} href={correctionPath}>进入修正</a>{/if}
        {/snippet}
      </InlineAlert>
    {/if}
  </div>
{/if}

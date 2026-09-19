<script lang="ts">
  import { onDestroy, tick } from 'svelte';

  import {
    checkSiteAvailability,
    siteAvailabilityTarget,
  } from '@/application/site-submission/site-submission.api.browser';
  import type { EditableSubmission } from '@/application/site-submission/site-submission.browser';
  import type { SiteSearchResult } from '@/application/site-submission/site-submission.types';
  import InlineAlert from '@/components/feedback/InlineAlert.svelte';

  interface Props {
    form: EditableSubmission;
    checkDuplicates: boolean;
    oncheckingchange?: (checking: boolean) => void;
  }

  type AvailabilityState = 'idle' | 'checking' | 'available' | 'duplicate' | 'failed';

  let { form = $bindable(), checkDuplicates, oncheckingchange }: Props = $props();
  let state = $state<AvailabilityState>('idle');
  let existingSite = $state.raw<SiteSearchResult | null>(null);
  let checkedURL = '';
  let requestSequence = 0;
  let controller: AbortController | null = null;

  onDestroy(() => {
    controller?.abort();
    oncheckingchange?.(false);
  });

  function setState(nextState: AvailabilityState): void {
    state = nextState;
    oncheckingchange?.(nextState === 'checking');
  }

  function isCheckableURL(value: string): boolean {
    try {
      const parsed = new URL(value);
      return parsed.protocol === 'http:' || parsed.protocol === 'https:';
    } catch {
      return false;
    }
  }

  function updateURL(event: Event): void {
    const input = event.currentTarget;
    if (!(input instanceof HTMLInputElement)) return;
    form.url = input.value;
    controller?.abort();
    requestSequence += 1;
    checkedURL = '';
    existingSite = null;
    setState('idle');
  }

  async function verifyAvailability(force = false): Promise<boolean> {
    if (!checkDuplicates) return true;
    const url = form.url.trim();
    if (!isCheckableURL(url)) return false;
    if (!force && checkedURL === url && state === 'available') return true;
    if (!force && checkedURL === url && state === 'duplicate') return false;

    controller?.abort();
    controller = new AbortController();
    const sequence = ++requestSequence;
    setState('checking');
    existingSite = null;
    try {
      const availability = await checkSiteAvailability(url, { signal: controller.signal });
      if (sequence !== requestSequence) return false;
      checkedURL = url;
      existingSite = availability.existing_site ?? null;
      setState(availability.available ? 'available' : 'duplicate');
      return availability.available;
    } catch (caught) {
      if (sequence !== requestSequence) return false;
      if (caught instanceof DOMException && caught.name === 'AbortError') return false;
      checkedURL = '';
      setState('failed');
      return false;
    }
  }

  export async function confirmAvailability(force = false): Promise<boolean> {
    const available = await verifyAvailability(force);
    if (!available) {
      await tick();
      document.querySelector<HTMLElement>('#site-address-availability [role="alert"]')?.focus();
    }
    return available;
  }
</script>

<div class="grid gap-4">
  <div>
    <h2 class="text-xl font-semibold">站点资料</h2>
    <p class="mt-1 text-sm text-fg-muted">填写站点在目录中展示的基础信息。</p>
  </div>
  <label class="grid gap-2 text-sm">
    站点名称
    <input
      class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
      bind:value={form.name}
      maxlength="160"
    />
  </label>
  <label class="grid gap-2 text-sm">
    主页地址
    <input
      class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
      value={form.url}
      aria-describedby={checkDuplicates && state !== 'idle' && state !== 'available'
        ? 'site-address-availability'
        : undefined}
      oninput={updateURL}
    />
  </label>
  {#if checkDuplicates && state !== 'idle' && state !== 'available'}
    <div id="site-address-availability" aria-busy={state === 'checking'}>
      {#if state === 'checking'}
        <p class="text-sm text-fg-muted" role="status">正在检查站点地址…</p>
      {:else if state === 'duplicate' && existingSite}
        <InlineAlert tone="warning">
          <p>
            {existingSite.visibility === 'REMOVED'
              ? '该站点已移除，请提交恢复申请。'
              : '该站点已在目录中，请改为提交更新申请。'}
          </p>
          {#snippet actions()}
            <a
              class="inline-flex min-h-11 w-fit items-center font-semibold underline underline-offset-4 sm:min-h-10"
              href={siteAvailabilityTarget(existingSite)}
            >
              {existingSite.visibility === 'REMOVED' ? '申请恢复站点' : '填写更新申请'}
            </a>
          {/snippet}
        </InlineAlert>
      {:else if state === 'failed'}
        <InlineAlert tone="danger">
          <p>暂时无法检查站点地址，请稍后重试。</p>
          {#snippet actions()}
            <button
              class="inline-flex min-h-11 items-center font-semibold underline underline-offset-4 sm:min-h-10"
              type="button"
              onclick={() => confirmAvailability(true)}
            >
              重新检查
            </button>
          {/snippet}
        </InlineAlert>
      {/if}
    </div>
  {/if}
  <label class="grid gap-2 text-sm">
    站点简介
    <textarea
      class="min-h-28 rounded-sm border border-line-strong bg-surface p-3"
      bind:value={form.summary}
      maxlength="2000"></textarea>
  </label>
</div>

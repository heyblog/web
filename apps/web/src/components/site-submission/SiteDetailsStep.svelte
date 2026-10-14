<script lang="ts">
  import type { SiteAvailabilityController } from '@/application/site-submission/site-availability.controller.svelte';
  import type { EditableSubmission } from '@/application/site-submission/site-submission.browser';

  import SiteAddressAvailability from './SiteAddressAvailability.svelte';

  interface Props {
    form: EditableSubmission;
    availability: SiteAvailabilityController;
    purpose: 'CREATE' | 'UPDATE';
    oncheck: () => Promise<boolean>;
  }
  let { form = $bindable(), availability, purpose, oncheck }: Props = $props();
  let addressInput: HTMLInputElement;
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
      bind:this={addressInput}
      bind:value={form.url}
      aria-invalid={availability.state === 'duplicate' || availability.state === 'invalid'}
      aria-describedby={availability.state !== 'idle' ? 'site-address-availability' : undefined}
      oninput={() => availability.clear()}
    />
  </label>
  <SiteAddressAvailability
    id="site-address-availability"
    {availability}
    {purpose}
    onretry={oncheck}
    onedit={() => addressInput.focus()}
  />
  <label class="grid gap-2 text-sm">
    站点简介
    <textarea
      class="min-h-28 rounded-sm border border-line-strong bg-surface p-3"
      bind:value={form.summary}
      maxlength="2000"></textarea>
  </label>
</div>

<script lang="ts">
  import { IconAlertTriangle, IconCircleX } from '@tabler/icons-svelte';
  import type { Snippet } from 'svelte';

  interface Props {
    id?: string;
    tone: 'warning' | 'danger';
    children: Snippet;
    actions?: Snippet;
  }

  const toneClasses = {
    warning: 'border-l-warning-border bg-warning-bg text-warning-fg',
    danger: 'border-l-danger bg-danger-bg text-danger-fg',
  } as const;

  let { id, tone, children, actions }: Props = $props();
  const Icon = $derived(tone === 'warning' ? IconAlertTriangle : IconCircleX);
</script>

<div
  {id}
  class={`grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-start gap-3 rounded-md border border-l-3 border-line p-4 text-sm ${toneClasses[tone]}`}
  role="alert"
  tabindex="-1"
  data-submission-alert
>
  <Icon class="mt-0.5 shrink-0" size={18} stroke={1.8} aria-hidden="true" />
  <div class="min-w-0">
    <div class="wrap-break-word">{@render children()}</div>
    {#if actions}
      <div class="mt-2 flex flex-wrap gap-3">{@render actions()}</div>
    {/if}
  </div>
</div>

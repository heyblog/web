<script lang="ts">
  import { IconFileText } from '@tabler/icons-svelte';

  import SiteClickCount from '@/components/site-profile/SiteClickCount.svelte';

  import type { BlogCardPlannedFields, BlogCardUpdateTone } from './blog-card-layout.shared';

  type Props = {
    readonly plannedFields?: BlogCardPlannedFields;
    readonly clickCount: number;
  };

  const updateToneClasses = {
    emerald: 'bg-success-bg text-success-fg',
    amber: 'bg-warning-bg text-warning-fg',
    blue: 'bg-info-bg text-info-fg',
    stone: 'bg-subtle text-fg-muted',
  } as const satisfies Record<BlogCardUpdateTone, string>;
  const numberFormat = new Intl.NumberFormat('zh-CN');

  let { plannedFields, clickCount }: Props = $props();
  const visible = $derived(
    plannedFields?.articleCount !== undefined || plannedFields?.contentUpdated !== undefined,
  );
</script>

<div class="mt-2 flex h-5 items-center gap-3 overflow-hidden text-xs text-fg-muted">
  <SiteClickCount count={clickCount} />
  {#if visible}
    {#if plannedFields?.articleCount !== undefined}
      <span
        class="inline-flex shrink-0 items-center gap-1 tabular-nums"
        aria-label={`${numberFormat.format(plannedFields.articleCount)} 篇文章`}
      >
        <IconFileText aria-hidden="true" size={14} stroke={1.8} />
        {numberFormat.format(plannedFields.articleCount)}
      </span>
    {/if}
    {#if plannedFields?.contentUpdated}
      <span
        class={`truncate rounded-sm px-1.5 py-0.5 font-medium ${updateToneClasses[plannedFields.contentUpdated.tone]}`}
      >
        {plannedFields.contentUpdated.label}
      </span>
    {/if}
  {/if}
</div>

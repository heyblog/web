<script lang="ts">
  import { IconChevronDown, IconFilter } from '@tabler/icons-svelte';

  import type {
    SiteDirectoryFeed,
    SiteDirectoryFilterName,
    SiteDirectoryOptions,
    SiteDirectoryQuery,
  } from '@/application/site-directory/site-directory.models';

  import SiteDirectoryFilters from './SiteDirectoryFilters.svelte';

  type Props = {
    readonly options: SiteDirectoryOptions;
    readonly query: SiteDirectoryQuery;
    readonly onToggle: (name: SiteDirectoryFilterName, value: string, selected: boolean) => void;
    readonly onClassificationChange: (level1: string, level2: string) => void;
    readonly onFeedChange: (feed: SiteDirectoryFeed) => void;
  };

  let { options, query, onToggle, onClassificationChange, onFeedChange }: Props = $props();
  let expanded = $state(false);
  const id = $props.id();
  const selectedCount = $derived(
    (query.level1 ? 1 : 0) +
      (query.level2 ? 1 : 0) +
      query.tertiary.length +
      query.warning.length +
      query.technology.length +
      query.access.length +
      (query.feed === 'any' ? 0 : 1),
  );
</script>

<aside
  class="min-w-0 rounded-md border border-line bg-surface lg:sticky lg:top-24 lg:self-start"
  aria-label="博客筛选"
>
  <button
    class="flex min-h-11 w-full items-center gap-2 rounded-md px-4 text-sm font-medium hover:bg-subtle lg:hidden"
    type="button"
    aria-expanded={expanded}
    aria-controls={id}
    onclick={() => (expanded = !expanded)}
  >
    <IconFilter size={16} stroke={1.8} aria-hidden="true" />
    <span>筛选目录</span>
    <span class="ml-auto text-xs text-fg-muted">
      {selectedCount > 0 ? `已选 ${selectedCount}` : expanded ? '收起筛选' : '展开筛选'}
    </span>
    <IconChevronDown
      class={[
        'shrink-0 transition-transform duration-(--motion-base) motion-reduce:transition-none',
        expanded && 'rotate-180',
      ]}
      size={16}
      stroke={1.8}
      aria-hidden="true"
    />
  </button>
  <div class="hidden items-center justify-between gap-3 border-b border-line px-4 py-3 lg:flex">
    <h2 class="text-sm font-semibold">筛选</h2>
    {#if selectedCount > 0}
      <span class="text-xs text-tint-fg">已选 {selectedCount}</span>
    {/if}
  </div>
  <div
    {id}
    class={[
      'min-w-0 border-t border-line p-3 lg:max-h-[calc(100dvh-10rem)] lg:overflow-y-auto lg:overscroll-contain lg:border-t-0',
      expanded ? 'block' : 'hidden lg:block',
    ]}
  >
    <SiteDirectoryFilters {options} {query} {onToggle} {onClassificationChange} {onFeedChange} />
  </div>
</aside>

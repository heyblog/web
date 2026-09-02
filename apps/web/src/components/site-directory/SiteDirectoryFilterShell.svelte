<script lang="ts">
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
    readonly onFeedChange: (feed: SiteDirectoryFeed) => void;
  };

  let { options, query, onToggle, onFeedChange }: Props = $props();
  const selectedCount = $derived(
    query.primary.length +
      query.secondary.length +
      query.warning.length +
      query.technology.length +
      query.access.length +
      (query.feed === 'any' ? 0 : 1),
  );
</script>

<details class="rounded-md border border-line bg-surface lg:hidden">
  <summary
    class="flex min-h-11 cursor-pointer list-none items-center justify-between gap-3 px-4 text-sm font-semibold"
  >
    <span>筛选目录</span>
    <span class="text-xs font-normal text-fg-muted">
      {selectedCount > 0 ? `已选 ${selectedCount}` : '展开筛选'}
    </span>
  </summary>
  <div class="border-t border-line p-3">
    <SiteDirectoryFilters {options} {query} {onToggle} {onFeedChange} embedded />
  </div>
</details>

<div class="hidden lg:sticky lg:top-24 lg:block lg:self-start">
  <SiteDirectoryFilters {options} {query} {onToggle} {onFeedChange} />
</div>

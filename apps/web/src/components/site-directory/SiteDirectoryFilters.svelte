<script lang="ts">
  import type {
    SiteDirectoryAccess,
    SiteDirectoryFeed,
    SiteDirectoryFilterName,
    SiteDirectoryOptions,
    SiteDirectoryQuery,
  } from '@/application/site-directory/site-directory.models';

  type Props = {
    readonly options: SiteDirectoryOptions;
    readonly query: SiteDirectoryQuery;
    readonly onToggle: (name: SiteDirectoryFilterName, value: string, selected: boolean) => void;
    readonly onFeedChange: (feed: SiteDirectoryFeed) => void;
    readonly embedded?: boolean;
  };

  const accessOptions = [
    { value: 'ALL', label: '全球可访问' },
    { value: 'CN_ONLY', label: '仅中国大陆' },
    { value: 'GLOBAL_ONLY', label: '仅海外' },
  ] as const satisfies readonly { value: SiteDirectoryAccess; label: string }[];
  let { options, query, onToggle, onFeedChange, embedded = false }: Props = $props();
  const selectedCount = $derived(
    query.primary.length +
      query.secondary.length +
      query.warning.length +
      query.technology.length +
      query.access.length +
      (query.feed === 'any' ? 0 : 1),
  );
  const optionCount = (normalCount: number, abnormalCount: number): number =>
    query.status === 'normal' ? normalCount : abnormalCount;
  const parseFeed = (value: string): SiteDirectoryFeed =>
    value === 'with' || value === 'without' ? value : 'any';
</script>

<aside
  class={embedded ? '' : 'rounded-md border border-line bg-surface p-4 shadow-2xs'}
  aria-label="博客筛选"
>
  {#if !embedded}
    <div class="flex items-center justify-between gap-3 border-b border-line pb-3">
      <h2 class="text-sm font-semibold">筛选</h2>
      {#if selectedCount > 0}
        <span class="rounded-sm bg-tint px-2 py-1 text-xs font-medium text-tint-fg">
          已选 {selectedCount}
        </span>
      {/if}
    </div>
  {/if}

  <div class="grid gap-1 pt-2">
    <details open={query.primary.length > 0}>
      <summary
        class="flex min-h-11 cursor-pointer list-none items-center justify-between rounded-sm px-2 text-sm font-medium hover:bg-subtle sm:min-h-10"
      >
        主标签
        <span class="text-xs text-fg-muted">
          {query.primary.length || options.primaryTags.length}
        </span>
      </summary>
      <div class="grid max-h-56 gap-1 overflow-y-auto px-2 pb-3">
        {#each options.primaryTags as option (option.value)}
          <label class="flex min-h-11 cursor-pointer items-center gap-2 text-sm sm:min-h-10">
            <input
              class="size-4 rounded-sm border-line-strong text-primary focus:ring-focus"
              type="checkbox"
              checked={query.primary.includes(option.value)}
              onchange={(event) => onToggle('primary', option.value, event.currentTarget.checked)}
            />
            <span class="min-w-0 flex-1 truncate">{option.label}</span>
            <span class="font-mono text-xs text-fg-muted">
              {optionCount(option.normalCount, option.abnormalCount)}
            </span>
          </label>
        {/each}
      </div>
    </details>

    <details open={query.secondary.length > 0}>
      <summary
        class="flex min-h-11 cursor-pointer list-none items-center justify-between rounded-sm px-2 text-sm font-medium hover:bg-subtle sm:min-h-10"
      >
        子标签（需全部匹配）
        <span class="text-xs text-fg-muted">
          {query.secondary.length || options.secondaryTags.length}
        </span>
      </summary>
      <div class="grid max-h-56 gap-1 overflow-y-auto px-2 pb-3">
        {#each options.secondaryTags as option (option.value)}
          <label class="flex min-h-11 cursor-pointer items-center gap-2 text-sm sm:min-h-10">
            <input
              class="size-4 rounded-sm border-line-strong text-primary focus:ring-focus"
              type="checkbox"
              checked={query.secondary.includes(option.value)}
              onchange={(event) => onToggle('secondary', option.value, event.currentTarget.checked)}
            />
            <span class="min-w-0 flex-1 truncate">{option.label}</span>
            <span class="font-mono text-xs text-fg-muted">
              {optionCount(option.normalCount, option.abnormalCount)}
            </span>
          </label>
        {/each}
      </div>
    </details>

    <details open={query.warning.length > 0}>
      <summary
        class="flex min-h-11 cursor-pointer list-none items-center justify-between rounded-sm px-2 text-sm font-medium hover:bg-subtle sm:min-h-10"
      >
        访问提示
        <span class="text-xs text-fg-muted">{query.warning.length || options.warnings.length}</span>
      </summary>
      <div class="grid max-h-44 gap-1 overflow-y-auto px-2 pb-3">
        {#each options.warnings as option (option.value)}
          <label class="flex min-h-11 cursor-pointer items-center gap-2 text-sm sm:min-h-10">
            <input
              class="size-4 rounded-sm border-line-strong text-primary focus:ring-focus"
              type="checkbox"
              checked={query.warning.includes(option.value)}
              onchange={(event) => onToggle('warning', option.value, event.currentTarget.checked)}
            />
            <span class="min-w-0 flex-1 truncate">{option.label}</span>
            <span class="font-mono text-xs text-fg-muted">
              {optionCount(option.normalCount, option.abnormalCount)}
            </span>
          </label>
        {/each}
      </div>
    </details>

    <details open={query.technology.length > 0}>
      <summary
        class="flex min-h-11 cursor-pointer list-none items-center justify-between rounded-sm px-2 text-sm font-medium hover:bg-subtle sm:min-h-10"
      >
        技术组件
        <span class="text-xs text-fg-muted">
          {query.technology.length || options.technologies.length}
        </span>
      </summary>
      <div class="grid max-h-56 gap-1 overflow-y-auto px-2 pb-3">
        {#each options.technologies as option (option.value)}
          <label class="flex min-h-11 cursor-pointer items-center gap-2 text-sm sm:min-h-10">
            <input
              class="size-4 rounded-sm border-line-strong text-primary focus:ring-focus"
              type="checkbox"
              checked={query.technology.includes(option.value)}
              onchange={(event) =>
                onToggle('technology', option.value, event.currentTarget.checked)}
            />
            <span class="min-w-0 flex-1 truncate">{option.label}</span>
            <span class="font-mono text-xs text-fg-muted">
              {optionCount(option.normalCount, option.abnormalCount)}
            </span>
          </label>
        {/each}
      </div>
    </details>

    <details open={query.access.length > 0}>
      <summary
        class="flex min-h-11 cursor-pointer list-none items-center justify-between rounded-sm px-2 text-sm font-medium hover:bg-subtle sm:min-h-10"
      >
        访问范围
        <span class="text-xs text-fg-muted">{query.access.length || accessOptions.length}</span>
      </summary>
      <div class="grid gap-1 px-2 pb-3">
        {#each accessOptions as option (option.value)}
          <label class="flex min-h-11 cursor-pointer items-center gap-2 text-sm sm:min-h-10">
            <input
              class="size-4 rounded-sm border-line-strong text-primary focus:ring-focus"
              type="checkbox"
              checked={query.access.includes(option.value)}
              onchange={(event) => onToggle('access', option.value, event.currentTarget.checked)}
            />
            {option.label}
          </label>
        {/each}
      </div>
    </details>

    <label class="mt-2 grid gap-1.5 border-t border-line px-2 pt-4 text-sm font-medium">
      订阅源
      <select
        class="min-h-11 rounded-md border border-line-strong bg-surface px-3 text-sm transition-colors duration-(--motion-fast) outline-none focus:border-focus focus:ring-2 focus:ring-focus/30 sm:min-h-10"
        value={query.feed}
        onchange={(event) => onFeedChange(parseFeed(event.currentTarget.value))}
      >
        <option value="any">不限</option>
        <option value="with">有 Feed</option>
        <option value="without">无 Feed</option>
      </select>
    </label>
  </div>
</aside>

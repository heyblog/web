<script lang="ts">
  import type {
    SiteDirectoryAccess,
    SiteDirectoryFeed,
    SiteDirectoryFilterName,
    SiteDirectoryOption,
    SiteDirectoryOptions,
    SiteDirectoryQuery,
  } from '@/application/site-directory/site-directory.models';

  import SiteDirectoryFilterGroup from './SiteDirectoryFilterGroup.svelte';

  type Props = {
    readonly options: SiteDirectoryOptions;
    readonly query: SiteDirectoryQuery;
    readonly onToggle: (name: SiteDirectoryFilterName, value: string, selected: boolean) => void;
    readonly onClassificationChange: (level1: string, level2: string) => void;
    readonly onFeedChange: (feed: SiteDirectoryFeed) => void;
  };

  const accessOptions = [
    { value: 'ALL', label: '全球可访问' },
    { value: 'CN_ONLY', label: '仅中国大陆' },
    { value: 'GLOBAL_ONLY', label: '仅海外' },
  ] as const satisfies readonly { value: SiteDirectoryAccess; label: string }[];
  let { options, query, onToggle, onClassificationChange, onFeedChange }: Props = $props();
  const id = $props.id();
  let tertiarySearch = $state('');
  const selectedClassification = $derived(
    options.classifications.find((option) => option.value === query.level1),
  );
  const tertiaryOptions = $derived(
    options.tertiaryTags.filter((option) =>
      option.label.toLocaleLowerCase().includes(tertiarySearch.trim().toLocaleLowerCase()),
    ),
  );
  const optionCount = (option: SiteDirectoryOption): number =>
    query.status === 'normal' ? option.normalCount : option.abnormalCount;
  const parseFeed = (value: string): SiteDirectoryFeed =>
    value === 'with' || value === 'without' ? value : 'any';
  const optionClass =
    'flex min-h-11 min-w-0 cursor-pointer items-center gap-2 rounded-sm px-1 py-2 text-sm transition-colors duration-(--motion-fast) hover:bg-subtle sm:min-h-10';
  const inputClass = 'size-4 shrink-0 accent-primary';
</script>

{#snippet checkboxes(
  name: SiteDirectoryFilterName,
  choices: readonly SiteDirectoryOption[],
  selected: readonly string[],
)}
  <div class="grid max-h-56 min-w-0 auto-rows-max gap-1 overflow-y-auto">
    {#each choices as option (option.value)}
      <label class={optionClass}>
        <input
          class={inputClass}
          type="checkbox"
          checked={selected.includes(option.value)}
          onchange={(event) => onToggle(name, option.value, event.currentTarget.checked)}
        />
        <span class="min-w-0 flex-1 wrap-anywhere">{option.label}</span>
        <span class="shrink-0 font-mono text-xs text-fg-muted">{optionCount(option)}</span>
      </label>
    {:else}
      <p class="py-2 text-xs text-fg-muted">
        {name === 'tertiary' && tertiarySearch ? '未找到匹配标签' : '暂无可选项'}
      </p>
    {/each}
  </div>
{/snippet}

{#snippet classifications(choices: readonly SiteDirectoryOption[], level: 'level1' | 'level2')}
  <div class="grid max-h-56 min-w-0 auto-rows-max gap-1 overflow-y-auto">
    <label class={optionClass}>
      <input
        class={inputClass}
        type="radio"
        name={`${id}-${level}`}
        checked={!query[level]}
        onchange={() => onClassificationChange(level === 'level1' ? '' : query.level1, '')}
      />
      <span>不限</span>
    </label>
    {#each choices as option (option.value)}
      <label class={optionClass}>
        <input
          class={inputClass}
          type="radio"
          name={`${id}-${level}`}
          checked={query[level] === option.value}
          onchange={() =>
            onClassificationChange(
              level === 'level1' ? option.value : query.level1,
              level === 'level2' ? option.value : '',
            )}
        />
        <span class="min-w-0 flex-1 wrap-anywhere">{option.label}</span>
        <span class="shrink-0 font-mono text-xs text-fg-muted">{optionCount(option)}</span>
      </label>
    {/each}
  </div>
{/snippet}

<div class="grid min-w-0">
  <SiteDirectoryFilterGroup label="博客分类" count={query.level1 ? 1 : 0} expanded>
    {@render classifications(options.classifications, 'level1')}
  </SiteDirectoryFilterGroup>
  {#if selectedClassification}
    <SiteDirectoryFilterGroup label="二级分类" count={query.level2 ? 1 : 0} expanded>
      {@render classifications(selectedClassification.children, 'level2')}
    </SiteDirectoryFilterGroup>
  {/if}
  <SiteDirectoryFilterGroup
    label="三级标签"
    count={query.tertiary.length}
    expanded={query.tertiary.length > 0}
  >
    <p class="text-xs text-fg-muted">同时匹配所选标签</p>
    <label class="sr-only" for={`${id}-tertiary-search`}>搜索三级标签</label>
    <input
      id={`${id}-tertiary-search`}
      class="min-h-11 w-full min-w-0 rounded-md border border-line-strong bg-surface px-3 text-base focus:border-focus sm:min-h-10 sm:text-sm"
      bind:value={tertiarySearch}
      placeholder="搜索三级标签"
    />
    {@render checkboxes('tertiary', tertiaryOptions, query.tertiary)}
  </SiteDirectoryFilterGroup>
  <SiteDirectoryFilterGroup
    label="访问提示"
    count={query.warning.length}
    expanded={query.warning.length > 0}
  >
    {@render checkboxes('warning', options.warnings, query.warning)}
  </SiteDirectoryFilterGroup>
  <SiteDirectoryFilterGroup
    label="技术组件"
    count={query.technology.length}
    expanded={query.technology.length > 0}
  >
    {@render checkboxes('technology', options.technologies, query.technology)}
  </SiteDirectoryFilterGroup>
  <SiteDirectoryFilterGroup
    label="访问范围"
    count={query.access.length}
    expanded={query.access.length > 0}
  >
    {#each accessOptions as option (option.value)}
      <label class={optionClass}>
        <input
          class={inputClass}
          type="checkbox"
          checked={query.access.includes(option.value)}
          onchange={(event) => onToggle('access', option.value, event.currentTarget.checked)}
        />
        <span class="min-w-0 wrap-anywhere">{option.label}</span>
      </label>
    {/each}
  </SiteDirectoryFilterGroup>
  <label class="grid min-w-0 gap-2 px-2 pt-4 pb-1 text-sm font-medium">
    订阅源
    <select
      class="min-h-11 w-full min-w-0 rounded-md border border-line-strong bg-surface px-3 text-base focus:border-focus sm:min-h-10 sm:text-sm"
      value={query.feed}
      onchange={(event) => onFeedChange(parseFeed(event.currentTarget.value))}
    >
      <option value="any">不限</option>
      <option value="with">有 Feed</option>
      <option value="without">无 Feed</option>
    </select>
  </label>
</div>

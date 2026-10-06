<script lang="ts">
  import { IconX } from '@tabler/icons-svelte';

  import type {
    SiteDirectoryFilterName,
    SiteDirectoryOptions,
    SiteDirectoryQuery,
  } from '@/api/sites/site-directory.types';
  import {
    directoryOptionSelected,
    selectedDirectoryOption,
  } from '@/application/site-directory/site-directory.labels';

  type SelectedFilter = {
    readonly key: string;
    readonly name: SiteDirectoryFilterName | 'classification' | 'feed';
    readonly value: string;
    readonly label: string;
  };
  type Props = {
    readonly options: SiteDirectoryOptions;
    readonly query: SiteDirectoryQuery;
    readonly onRemove: (
      name: SiteDirectoryFilterName | 'classification' | 'feed',
      value: string,
    ) => void;
    readonly onClear: () => void;
  };

  let { options, query, onRemove, onClear }: Props = $props();
  const accessLabels = new Map([
    ['ALL', '全球可访问'],
    ['CN_ONLY', '仅中国大陆'],
    ['GLOBAL_ONLY', '仅海外'],
  ]);
  const labels = $derived(
    new Map([
      ...options.warnings.map((option) => [`warning:${option.value}`, option.label] as const),
      ...options.technologies.map(
        (option) => [`technology:${option.value}`, option.label] as const,
      ),
    ]),
  );
  const selected = $derived.by((): readonly SelectedFilter[] => {
    const result: SelectedFilter[] = [];
    if (query.level1) {
      result.push({
        key: `level1:${query.level1}`,
        name: 'classification',
        value: 'level1',
        label:
          selectedDirectoryOption(options.classifications, query.level1, query.level1_label_id)
            ?.label ?? query.level1,
      });
    }
    if (query.level2) {
      result.push({
        key: `level2:${query.level2}`,
        name: 'classification',
        value: 'level2',
        label:
          selectedDirectoryOption(
            options.classifications
              .filter((option) => option.value === query.level1)
              .flatMap((option) => option.children),
            query.level2,
            query.level2_label_id,
          )?.label ?? query.level2,
      });
    }
    const groups = [
      ['tertiary', query.tertiary],
      ['warning', query.warning],
      ['technology', query.technology],
      ['access', query.access],
    ] as const;
    for (const [name, values] of groups) {
      for (const value of values) {
        result.push({
          key: `${name}:${value}`,
          name,
          value,
          label:
            name === 'access'
              ? (accessLabels.get(value) ?? value)
              : name === 'tertiary'
                ? (options.tertiaryTags.find(
                    (option) =>
                      option.value === value &&
                      directoryOptionSelected(
                        options.tertiaryTags,
                        option,
                        query.tertiary,
                        query.tertiary_label_ids,
                      ),
                  )?.label ?? value)
                : (labels.get(`${name}:${value}`) ?? value),
        });
      }
    }
    if (query.feed !== 'any') {
      result.push({
        key: `feed:${query.feed}`,
        name: 'feed',
        value: query.feed,
        label: query.feed === 'with' ? '有 Feed' : '无 Feed',
      });
    }
    return result;
  });
</script>

{#if selected.length > 0}
  <section class="border-b border-line py-4" aria-labelledby="selected-filters-title">
    <div class="mb-2 flex items-center justify-between gap-3">
      <h2 class="text-xs font-medium text-fg-muted" id="selected-filters-title">已选筛选</h2>
      <button
        class="min-h-11 rounded-md px-2 text-xs font-medium text-fg-muted hover:bg-subtle hover:text-fg sm:min-h-10"
        type="button"
        onclick={onClear}
      >
        清空筛选
      </button>
    </div>
    <div class="flex flex-wrap gap-2">
      {#each selected as item (item.key)}
        <span
          class="inline-flex min-h-8 max-w-full min-w-0 items-center rounded-sm bg-tint pl-2 text-xs font-medium text-tint-fg"
        >
          <span class="min-w-0 wrap-anywhere">{item.label}</span>
          <button
            class="inline-flex min-h-11 min-w-11 shrink-0 items-center justify-center rounded-sm hover:bg-primary/10 sm:min-h-10 sm:min-w-10"
            type="button"
            aria-label={`移除筛选：${item.label}`}
            onclick={() => onRemove(item.name, item.value)}
          >
            <IconX aria-hidden="true" size={13} stroke={2} />
          </button>
        </span>
      {/each}
    </div>
  </section>
{/if}

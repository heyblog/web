<script lang="ts">
  import { IconArrowsShuffle, IconChevronLeft, IconChevronRight } from '@tabler/icons-svelte-runes';

  import { mapDirectoryItemToSiteCardEntry } from '@/application/site/site-card.shared';
  import type {
    SiteDirectoryItem,
    SiteDirectoryResult,
  } from '@/application/site/site-directory.models';
  import BlogCard from '@/components/site/BlogCard.svelte';
  import {
    formatDirectoryCount,
    SITE_DIRECTORY_SORT_OPTIONS,
  } from '@/components/site/site-directory-page.shared';
  import SiteDirectoryFilterPopover from '@/components/site/SiteDirectoryFilterPopover.svelte';

  let {
    result,
    pending = false,
    sortSummary,
    onStatusModeChange,
    onRandomToggle,
    onSortChange,
    onOrderToggle,
    onChangePage,
    onFeedback,
  }: {
    result: SiteDirectoryResult;
    pending?: boolean;
    sortSummary: string;
    onStatusModeChange?: (statusMode: 'normal' | 'abnormal') => void;
    onRandomToggle?: () => void;
    onSortChange?: (value: string) => void;
    onOrderToggle?: () => void;
    onChangePage?: (page: number) => void;
    onFeedback?: (item: SiteDirectoryItem) => void;
  } = $props();

  const directoryCards = $derived(
    result.items.map((item) => mapDirectoryItemToSiteCardEntry(item)),
  );
</script>

<section class="page-section">
  <div class="flex flex-wrap items-end justify-between gap-3">
    <div>
      <div class="flex flex-wrap items-center gap-2">
        <p class="font-mono text-[11px] tracking-[0.16em] text-(--color-fg-3) uppercase">Results</p>
        <div class="inline-flex items-center rounded-md border border-(--color-line-med) p-0.5">
          <button
            class={`inline-flex items-center rounded-sm px-2.5 py-1.5 text-sm transition ${
              result.query.statusMode === 'normal'
                ? 'bg-(--color-bg-raised) text-(--color-fg)'
                : 'text-(--color-fg-3) hover:text-(--color-fg)'
            }`}
            type="button"
            onclick={() => onStatusModeChange?.('normal')}
          >
            正常站点
          </button>
          <button
            class={`inline-flex items-center rounded-sm px-2.5 py-1.5 text-sm transition ${
              result.query.statusMode === 'abnormal'
                ? 'bg-(--color-bg-raised) text-(--color-fg)'
                : 'text-(--color-fg-3) hover:text-(--color-fg)'
            }`}
            type="button"
            onclick={() => onStatusModeChange?.('abnormal')}
          >
            异常站点
          </button>
        </div>
      </div>
      <p class="mt-2 text-sm text-(--color-fg-2)">
        共 {formatDirectoryCount(result.pagination.totalItems)} 个站点，第 {result.pagination.page} /
        {result.pagination.totalPages} 页
      </p>
    </div>

    <div class="flex flex-wrap items-center justify-end gap-2">
      <button
        class={`inline-flex h-8 items-center gap-1.5 rounded-sm border px-2.5 text-sm transition ${
          result.query.random && !result.query.sort
            ? 'border-[color-mix(in_srgb,var(--color-info)_22%,var(--color-line-med))] bg-[color-mix(in_srgb,var(--color-info)_6%,var(--color-bg))] text-(--color-info)'
            : 'border-(--color-line-med) text-(--color-fg-2) hover:bg-(--color-bg-raised) hover:text-(--color-fg)'
        }`}
        type="button"
        onclick={onRandomToggle}
      >
        <IconArrowsShuffle size={14} stroke={1.8} />
        <span>{result.query.random && !result.query.sort ? '稳定随机' : '随机排序'}</span>
      </button>

      <SiteDirectoryFilterPopover
        label="排序"
        title="选择排序方式"
        items={SITE_DIRECTORY_SORT_OPTIONS.map((item) => ({
          key: item.key,
          label: item.label,
        }))}
        selected={[result.query.sort ?? '']}
        multiple={false}
        searchable={false}
        triggerVariant="ghost"
        selectedSummary={sortSummary}
        widthClass="w-[12rem]"
        onToggle={onSortChange}
      />

      {#if result.query.sort}
        <button
          class="inline-flex h-6 items-center rounded-sm px-0 text-sm text-(--color-fg-3) transition hover:text-(--color-fg)"
          type="button"
          onclick={onOrderToggle}
        >
          {result.query.order === 'asc' ? '升序' : '降序'}
        </button>
      {/if}
    </div>
  </div>

  {#if result.items.length > 0}
    <div class="mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {#each directoryCards as entry, index}
        <BlogCard {entry} onFeedback={() => onFeedback?.(result.items[index]!)} />
      {/each}
    </div>
  {:else}
    <div
      class="mt-6 rounded-md border border-dashed border-(--color-line-med) px-5 py-8 text-sm text-(--color-fg-2)"
    >
      没有匹配的站点。
    </div>
  {/if}
  {#if result.items.length > 0}
    <div class="mt-6 flex items-center justify-end gap-2">
      <button
        class="inline-flex h-10 items-center gap-2 rounded-md border border-(--color-line-med) px-3 text-sm text-(--color-fg-2) transition hover:bg-(--color-bg-raised) hover:text-(--color-fg) disabled:cursor-not-allowed disabled:opacity-40"
        type="button"
        disabled={result.pagination.page <= 1 || pending}
        onclick={() => onChangePage?.(result.pagination.page - 1)}
      >
        <IconChevronLeft size={15} stroke={1.8} />
        <span>上一页</span>
      </button>
      <button
        class="inline-flex h-10 items-center gap-2 rounded-md border border-(--color-line-med) px-3 text-sm text-(--color-fg-2) transition hover:bg-(--color-bg-raised) hover:text-(--color-fg) disabled:cursor-not-allowed disabled:opacity-40"
        type="button"
        disabled={result.pagination.page >= result.pagination.totalPages || pending}
        onclick={() => onChangePage?.(result.pagination.page + 1)}
      >
        <span>下一页</span>
        <IconChevronRight size={15} stroke={1.8} />
      </button>
    </div>
  {/if}
</section>

<script lang="ts">
  import { IconChevronLeft, IconChevronRight } from '@tabler/icons-svelte';

  import type { SiteDirectoryView } from '@/application/site-directory/site-directory.models';
  import BlogCard from '@/components/home/BlogCard.svelte';

  type Props = {
    readonly view: SiteDirectoryView;
    readonly pending: boolean;
    readonly onPageChange: (page: number) => void;
  };

  let { view, pending, onPageChange }: Props = $props();
  let expandedSiteId = $state<string | null>(null);
</script>

<section aria-labelledby="directory-results-title" aria-busy={pending}>
  <div class="flex flex-wrap items-end justify-between gap-3 border-b border-line pb-4">
    <div>
      <p class="font-mono text-xs font-medium text-tint-fg">目录结果</p>
      <h2 class="mt-1 text-xl/snug font-semibold" id="directory-results-title">
        {view.pagination.totalItems.toLocaleString('zh-CN')} 个博客
      </h2>
    </div>
    <p class="text-sm text-fg-muted">
      第 {view.pagination.page} / {view.pagination.totalPages} 页
    </p>
  </div>

  {#if view.items.length > 0}
    <div
      class={[
        'mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3',
        pending && 'pointer-events-none opacity-60',
      ]}
      id="site-directory-results"
    >
      {#each view.items as site (site.shortId)}
        <BlogCard
          {site}
          expanded={expandedSiteId === site.shortId}
          onExpand={() => (expandedSiteId = site.shortId)}
          onClose={() => {
            if (expandedSiteId === site.shortId) expandedSiteId = null;
          }}
        />
      {/each}
    </div>
  {:else}
    <div
      class="mt-6 flex min-h-56 flex-col items-center justify-center rounded-md border border-dashed border-line-strong bg-surface px-6 text-center"
    >
      <span
        class="inline-flex size-12 items-center justify-center rounded-md bg-subtle font-mono text-sm font-semibold text-fg-muted"
        aria-hidden="true"
      >
        HB
      </span>
      <h3 class="mt-4 text-sm font-semibold">没有匹配的博客</h3>
      <p class="mt-2 text-sm text-fg-muted">调整关键词或筛选条件后再试。</p>
    </div>
  {/if}

  {#if view.pagination.totalPages > 1}
    <nav class="mt-8 flex items-center justify-end gap-2" aria-label="博客列表分页">
      <button
        class="inline-flex min-h-11 items-center gap-1.5 rounded-md border border-line-strong bg-surface px-3 text-sm font-medium text-fg-muted transition-[border-color,background-color,color,scale] duration-(--motion-fast) hover:border-primary hover:bg-tint hover:text-tint-fg active:scale-96 disabled:pointer-events-none disabled:opacity-40 sm:min-h-10"
        type="button"
        disabled={pending || view.pagination.page <= 1}
        onclick={() => onPageChange(view.pagination.page - 1)}
      >
        <IconChevronLeft aria-hidden="true" size={16} stroke={1.8} />
        上一页
      </button>
      <button
        class="inline-flex min-h-11 items-center gap-1.5 rounded-md border border-line-strong bg-surface px-3 text-sm font-medium text-fg-muted transition-[border-color,background-color,color,scale] duration-(--motion-fast) hover:border-primary hover:bg-tint hover:text-tint-fg active:scale-96 disabled:pointer-events-none disabled:opacity-40 sm:min-h-10"
        type="button"
        disabled={pending || view.pagination.page >= view.pagination.totalPages}
        onclick={() => onPageChange(view.pagination.page + 1)}
      >
        下一页
        <IconChevronRight aria-hidden="true" size={16} stroke={1.8} />
      </button>
    </nav>
  {/if}
</section>

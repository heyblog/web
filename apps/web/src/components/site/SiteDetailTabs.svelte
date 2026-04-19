<script lang="ts">
  import { trackSiteAccess } from '@/application/site/site-access.client';
  import type {
    PagedResult,
    SiteArticleItem,
    SiteCheckItem,
  } from '@/application/site/site-directory.models';
  import {
    buildSiteCheckFacts,
    formatSiteCheckRegionLabel,
    formatSiteCheckResultLabel,
    formatSiteDetailDateTime,
    resolveSiteCheckTextClass,
  } from '@/components/site/site-detail.shared';

  let {
    siteId,
    activeTab,
    articles,
    checks,
    loadingTab = null,
    onTabChange,
    onLoadPage,
  }: {
    siteId: string;
    activeTab: 'articles' | 'checks' | 'history';
    articles: PagedResult<SiteArticleItem>;
    checks: PagedResult<SiteCheckItem>;
    loadingTab?: 'articles' | 'checks' | null;
    onTabChange?: (tab: 'articles' | 'checks' | 'history') => void;
    onLoadPage?: (tab: 'articles' | 'checks', page: number) => void;
  } = $props();
</script>

<section class="page-section border-t border-(--color-line) pt-3">
  <div class="flex flex-wrap items-center gap-2 border-b border-(--color-line) pb-3">
    {#each [{ key: 'articles', label: 'RSS 文章' }, { key: 'checks', label: '可访问性检测' }, { key: 'history', label: '时间线' }] as tab (tab.key)}
      <button
        class={`rounded-md px-3 py-2 text-sm transition ${
          activeTab === tab.key
            ? 'bg-(--color-bg-raised) text-(--color-fg)'
            : 'text-(--color-fg-2) hover:text-(--color-fg)'
        }`}
        type="button"
        onclick={() => onTabChange?.(tab.key as 'articles' | 'checks' | 'history')}
      >
        {tab.label}
      </button>
    {/each}
  </div>

  {#if activeTab === 'articles'}
    <div class="mt-5 space-y-3">
      {#if articles.items.length > 0}
        <div class="overflow-hidden rounded-md border border-(--color-line)">
          {#each articles.items as article, index (article.id)}
            <article class={`px-4 py-4 ${index > 0 ? 'border-t border-(--color-line)' : ''}`}>
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <a
                    class="text-[16px] leading-7 text-(--color-fg) transition hover:text-(--color-info)"
                    href={article.articleUrl}
                    rel="noreferrer"
                    target="_blank"
                    onclick={() => {
                      trackSiteAccess(siteId, {
                        source: 'SITE_DETAIL',
                        targetKind: 'ARTICLE',
                      });
                    }}
                  >
                    {article.title}
                  </a>
                  <p class="mt-2 text-sm text-(--color-fg-3)">
                    {formatSiteDetailDateTime(article.publishedTime)} · {article.source.feedName ??
                      'RSS'}
                  </p>
                </div>
              </div>
              {#if article.summary}
                <p class="mt-3 text-sm leading-7 text-(--color-fg-2)">{article.summary}</p>
              {/if}
            </article>
          {/each}
        </div>
      {:else}
        <div
          class="rounded-md border border-dashed border-(--color-line-med) px-5 py-8 text-sm text-(--color-fg-2)"
        >
          暂无 RSS 文章。
        </div>
      {/if}

      {#if articles.items.length > 0}
        <div class="flex justify-end gap-2">
          <button
            class="rounded-md border border-(--color-line-med) px-3 py-2 text-sm text-(--color-fg-2) transition hover:bg-(--color-bg-raised) hover:text-(--color-fg) disabled:opacity-40"
            type="button"
            disabled={articles.pagination.page <= 1 || loadingTab === 'articles'}
            onclick={() => onLoadPage?.('articles', articles.pagination.page - 1)}
          >
            上一页
          </button>
          <button
            class="rounded-md border border-(--color-line-med) px-3 py-2 text-sm text-(--color-fg-2) transition hover:bg-(--color-bg-raised) hover:text-(--color-fg) disabled:opacity-40"
            type="button"
            disabled={articles.pagination.page >= articles.pagination.totalPages ||
              loadingTab === 'articles'}
            onclick={() => onLoadPage?.('articles', articles.pagination.page + 1)}
          >
            下一页
          </button>
        </div>
      {/if}
    </div>
  {/if}

  {#if activeTab === 'checks'}
    <div class="mt-5 space-y-3">
      {#if checks.items.length > 0}
        <div class="overflow-hidden rounded-md border border-(--color-line)">
          {#each checks.items as item, index (item.id)}
            <article class={`px-4 py-4 ${index > 0 ? 'border-t border-(--color-line)' : ''}`}>
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p class="text-[15px] font-medium text-(--color-fg)">
                    {formatSiteCheckRegionLabel(item.region)}
                  </p>
                  <p class="mt-1 text-sm text-(--color-fg-3)">
                    {formatSiteDetailDateTime(item.checkTime)}
                  </p>
                </div>
                <p class={`text-sm font-medium ${resolveSiteCheckTextClass(item.result)}`}>
                  {formatSiteCheckResultLabel(item.result)}
                </p>
              </div>
              <div class="mt-3 grid gap-2 text-sm text-(--color-fg-2) sm:grid-cols-2">
                {#each buildSiteCheckFacts(item) as fact (`${fact.label}:${fact.value}`)}
                  <p>{fact.label}：{fact.value}</p>
                {/each}
              </div>
              {#if item.message}
                <p class="mt-3 text-sm leading-7 text-(--color-fg-2)">{item.message}</p>
              {/if}
            </article>
          {/each}
        </div>
      {:else}
        <div
          class="rounded-md border border-dashed border-(--color-line-med) px-5 py-8 text-sm text-(--color-fg-2)"
        >
          暂无检测记录。
        </div>
      {/if}

      {#if checks.items.length > 0}
        <div class="flex justify-end gap-2">
          <button
            class="rounded-md border border-(--color-line-med) px-3 py-2 text-sm text-(--color-fg-2) transition hover:bg-(--color-bg-raised) hover:text-(--color-fg) disabled:opacity-40"
            type="button"
            disabled={checks.pagination.page <= 1 || loadingTab === 'checks'}
            onclick={() => onLoadPage?.('checks', checks.pagination.page - 1)}
          >
            上一页
          </button>
          <button
            class="rounded-md border border-(--color-line-med) px-3 py-2 text-sm text-(--color-fg-2) transition hover:bg-(--color-bg-raised) hover:text-(--color-fg) disabled:opacity-40"
            type="button"
            disabled={checks.pagination.page >= checks.pagination.totalPages ||
              loadingTab === 'checks'}
            onclick={() => onLoadPage?.('checks', checks.pagination.page + 1)}
          >
            下一页
          </button>
        </div>
      {/if}
    </div>
  {/if}

  {#if activeTab === 'history'}
    <div
      class="mt-5 rounded-md border border-dashed border-(--color-line-med) px-5 py-8 text-sm text-(--color-fg-2)"
    >
      <!-- // TODO: 时间线功能开发中，敬请期待。 -->
      时间线功能开发中，敬请期待。
    </div>
  {/if}
</section>

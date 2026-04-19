<script lang="ts">
  import type {
    SiteSubscriptionItem,
    SiteSubscriptionResult,
  } from '@/application/site/site-subscription.models';
  import { resolveSubscriptionIntro } from '@/components/site/site-subscription.shared';
  import FormMessage from '@/shared/ui/FormMessage.svelte';

  let { initialResult }: { initialResult: SiteSubscriptionResult } = $props();

  function cloneResult(value: SiteSubscriptionResult): SiteSubscriptionResult {
    return {
      summary: { ...value.summary },
      items: value.items.map((item) => ({
        ...item,
        source: { ...item.source },
        site: { ...item.site },
      })),
      pagination: { ...value.pagination },
    };
  }

  function createInitialResultValue(): SiteSubscriptionResult {
    return cloneResult(initialResult);
  }

  function formatDate(value: string | null): string {
    if (!value) {
      return '未提供';
    }

    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) {
      return value;
    }

    return parsed.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  function formatMetric(value: number): string {
    return new Intl.NumberFormat('zh-CN').format(value);
  }

  let result = $state(createInitialResultValue());
  let pending = $state(false);

  function syncPageUrl(page: number) {
    const url = new URL(window.location.href);

    if (page <= 1) {
      url.searchParams.delete('page');
    } else {
      url.searchParams.set('page', String(page));
    }

    if (result.pagination.pageSize === 20) {
      url.searchParams.delete('pageSize');
    } else {
      url.searchParams.set('pageSize', String(result.pagination.pageSize));
    }

    window.history.replaceState({}, '', `${url.pathname}${url.search}`);
  }

  async function loadPage(nextPage: number) {
    if (pending || nextPage < 1 || nextPage > result.pagination.totalPages) {
      return;
    }

    pending = true;

    try {
      const response = await fetch(
        `/api/subscriptions?page=${nextPage}&pageSize=${result.pagination.pageSize}`,
        { headers: { accept: 'application/json' } },
      );

      if (!response.ok) {
        return;
      }

      const payload = (await response.json()) as {
        ok: boolean;
        data: SiteSubscriptionResult;
      };

      result = cloneResult(payload.data);
      syncPageUrl(payload.data.pagination.page);
    } finally {
      pending = false;
    }
  }

  function resolveFeedLabel(item: SiteSubscriptionItem): string {
    return item.source.feedName ?? item.source.feedType ?? '默认订阅';
  }
</script>

<div class="page-shell">
  <section class="page-stack">
    <section class="page-section pt-2">
      <div class="section-head">
        <div>
          <p class="eyebrow">
            <span class="status-dot" style="--status-dot: var(--color-info-dot)"></span>
            Subscription
          </p>
          <h1 class="section-title text-[clamp(2rem,5vw,3.5rem)] leading-[0.96] tracking-[-0.04em]">
            订阅一览
          </h1>
          <p class="mt-4 max-w-3xl text-base leading-7 text-(--color-fg-2)">
            按发布时间查看当前已抓取的全部公开 RSS 文章。
          </p>
        </div>
      </div>

      <div class="mt-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {#each [{ label: '今日更新', value: result.summary.todayArticles }, { label: '近 7 天更新', value: result.summary.weekArticles }, { label: '文章总数', value: result.summary.totalArticles }, { label: '涉及站点', value: result.summary.siteCount }] as card (card.label)}
          <article class="rounded-md border border-(--color-line) bg-(--color-bg) px-4 py-4">
            <p class="font-mono text-[11px] uppercase tracking-[0.18em] text-(--color-fg-3)">
              {card.label}
            </p>
            <p class="mt-3 text-3xl font-medium tracking-[-0.04em] text-(--color-fg)">
              {formatMetric(card.value)}
            </p>
          </article>
        {/each}
      </div>
    </section>

    <section class="page-section space-y-4">
      {#if result.items.length === 0}
        <FormMessage
          tone="info"
          eyebrow="SUBSCRIPTION"
          title="暂无可展示的 RSS 文章"
          message="当前公开目录里还没有可用于订阅页展示的文章。"
        />
      {:else}
        {#each result.items as item (item.id)}
          {@const intro = resolveSubscriptionIntro(item.summary)}
          <article class="rounded-md border border-(--color-line) bg-(--color-bg) p-5">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div class="min-w-0">
                <a
                  class="text-[17px] leading-7 text-(--color-fg) transition hover:text-(--color-info)"
                  href={item.articleUrl}
                  rel="noreferrer"
                  target="_blank"
                >
                  {item.title}
                </a>
                <div
                  class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-(--color-fg-3)"
                >
                  <a class="transition hover:text-(--color-fg)" href={`/site/${item.site.slug}`}>
                    {item.site.name}
                  </a>
                  <span>{resolveFeedLabel(item)}</span>
                  <span>发布时间：{formatDate(item.publishedTime)}</span>
                  <span>抓取时间：{formatDate(item.fetchedTime)}</span>
                </div>
              </div>
            </div>

            {#if intro}
              <p class="mt-3 text-sm leading-7 text-(--color-fg-2)">简介：{intro}</p>
            {/if}
          </article>
        {/each}
      {/if}

      <div
        class="flex flex-wrap items-center justify-between gap-3 border-t border-(--color-line) pt-4 text-sm"
      >
        <p class="text-(--color-fg-3)">
          第 {result.pagination.page} / {result.pagination.totalPages} 页，共 {formatMetric(
            result.pagination.totalItems,
          )} 条
        </p>
        <div class="flex items-center gap-2">
          <button
            class="inline-flex items-center rounded-sm border border-(--color-line-med) px-3 py-2 disabled:cursor-not-allowed disabled:opacity-50"
            disabled={pending || result.pagination.page <= 1}
            onclick={() => loadPage(result.pagination.page - 1)}
            type="button"
          >
            上一页
          </button>
          <button
            class="inline-flex items-center rounded-sm border border-(--color-line-med) px-3 py-2 disabled:cursor-not-allowed disabled:opacity-50"
            disabled={pending || result.pagination.page >= result.pagination.totalPages}
            onclick={() => loadPage(result.pagination.page + 1)}
            type="button"
          >
            下一页
          </button>
        </div>
      </div>
    </section>
  </section>
</div>

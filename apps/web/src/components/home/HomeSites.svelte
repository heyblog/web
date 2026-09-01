<script lang="ts">
  import { IconRefresh } from '@tabler/icons-svelte';
  import { onDestroy } from 'svelte';

  import { refreshHome } from '@/application/home/home.browser';
  import type { HomeMockMode, HomeSiteCard } from '@/application/home/home.shared';

  import BlogCard from './BlogCard.svelte';

  interface Props {
    sites: HomeSiteCard[];
    unavailable?: boolean;
    mockMode?: HomeMockMode | null;
  }

  type RefreshStatus = 'idle' | 'loading' | 'error';

  let { sites, unavailable = false, mockMode = null }: Props = $props();

  let refreshStatus = $state<RefreshStatus>('idle');
  let expandedSiteId = $state<string | null>(null);
  let statusMessage = $state('');
  let refreshController: AbortController | undefined;
  const refreshLabel = $derived(
    refreshStatus === 'loading'
      ? '正在获取'
      : unavailable || refreshStatus === 'error'
        ? '重试'
        : '换一批',
  );

  onDestroy(() => refreshController?.abort());

  async function handleRefresh() {
    if (refreshStatus === 'loading') return;

    refreshStatus = 'loading';
    statusMessage = '';

    if (mockMode !== null) {
      refreshMockSites(mockMode);
      return;
    }

    refreshController = new AbortController();
    try {
      const home = await refreshHome(refreshController.signal);
      expandedSiteId = null;
      sites = home.sites;
      unavailable = false;
      refreshStatus = 'idle';
      statusMessage = sites.length === 0 ? '暂无可展示的博客资料' : '已换一批博客';
    } catch {
      refreshStatus = 'error';
      unavailable = sites.length === 0;
      statusMessage = '推荐暂时无法加载';
    } finally {
      refreshController = undefined;
    }
  }

  function refreshMockSites(mode: HomeMockMode) {
    expandedSiteId = null;
    if (mode === 'cards') {
      const offset = Math.min(3, Math.max(1, sites.length - 1));
      sites = [...sites.slice(offset), ...sites.slice(0, offset)];
      unavailable = false;
      refreshStatus = 'idle';
      statusMessage = '已换一批博客';
      return;
    }

    sites = [];
    unavailable = mode === 'error';
    refreshStatus = mode === 'error' ? 'error' : 'idle';
    statusMessage = mode === 'error' ? '推荐暂时无法加载' : '暂无可展示的博客资料';
  }
</script>

<section
  class="scroll-mt-24 py-12 sm:py-16"
  id="discover"
  aria-labelledby="discover-title"
  aria-busy={refreshStatus === 'loading'}
>
  <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
    <div>
      <p class="font-mono text-xs font-medium text-tint-fg">每日发现</p>
      <h2 class="mt-2 text-2xl/tight font-semibold" id="discover-title">今天，从这些博客开始</h2>
    </div>
    <button
      class="inline-flex min-h-11 shrink-0 items-center gap-1.5 self-start rounded-md px-3 text-sm font-medium text-fg-muted transition-[background-color,color,scale] duration-(--motion-fast) ease-standard hover:bg-subtle hover:text-fg active:scale-96 disabled:pointer-events-none disabled:opacity-50 motion-reduce:transition-colors sm:min-h-10 sm:self-auto"
      type="button"
      aria-controls="home-site-results"
      aria-busy={refreshStatus === 'loading'}
      disabled={refreshStatus === 'loading'}
      onclick={handleRefresh}
    >
      <IconRefresh
        class={['size-4', refreshStatus === 'loading' && 'animate-spin motion-reduce:animate-none']}
        aria-hidden="true"
        size={16}
        stroke={1.8}
      />
      {refreshLabel}
    </button>
  </div>

  <div id="home-site-results">
    {#if unavailable}
      <div
        class="mt-6 flex min-h-44 flex-col items-center justify-center rounded-md border border-line bg-surface px-6 text-center"
      >
        <span
          class="inline-flex size-12 items-center justify-center rounded-md bg-subtle text-fg-muted"
        >
          <IconRefresh aria-hidden="true" size={20} stroke={1.8} />
        </span>
        <h3 class="mt-4 text-sm font-semibold">推荐暂时无法加载</h3>
      </div>
    {:else if sites.length === 0}
      <div
        class="mt-6 flex min-h-44 flex-col items-center justify-center rounded-md border border-line bg-surface px-6 text-center"
      >
        <span
          class="inline-flex size-12 items-center justify-center rounded-md bg-subtle font-mono text-sm font-semibold text-fg-muted"
        >
          HB
        </span>
        <h3 class="mt-4 text-sm font-semibold">暂无可展示的博客资料</h3>
      </div>
    {:else}
      <div class="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3">
        {#each sites as site (site.shortId)}
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
    {/if}
  </div>

  <p class="sr-only" aria-live="polite" aria-atomic="true">{statusMessage}</p>
</section>

<script lang="ts">
  import { IconRefresh } from '@tabler/icons-svelte';
  import { onDestroy } from 'svelte';

  import { refreshSiteDirectory } from '@/application/site-directory/site-directory.browser';
  import type {
    SiteDirectoryAccess,
    SiteDirectoryFeed,
    SiteDirectoryFilterName,
    SiteDirectoryOptions,
    SiteDirectoryOrder,
    SiteDirectoryQuery,
    SiteDirectorySort,
    SiteDirectoryStatus,
    SiteDirectoryView,
  } from '@/application/site-directory/site-directory.models';
  import {
    buildSiteDirectorySearchParams,
    createDirectoryShuffleSeed,
    parseSiteDirectorySearchParams,
  } from '@/application/site-directory/site-directory.shared';

  import SiteDirectoryFilterShell from './SiteDirectoryFilterShell.svelte';
  import SiteDirectoryHero from './SiteDirectoryHero.svelte';
  import SiteDirectoryResults from './SiteDirectoryResults.svelte';
  import SiteDirectorySelectedFilters from './SiteDirectorySelectedFilters.svelte';
  import SiteDirectoryToolbar from './SiteDirectoryToolbar.svelte';

  type LoadCause =
    'search' | 'filter' | 'status' | 'sort' | 'shuffle' | 'page' | 'popstate' | 'retry';
  type Props = {
    readonly initialView: SiteDirectoryView;
    readonly options: SiteDirectoryOptions;
  };

  let { initialView, options }: Props = $props();
  const readInitialView = (): SiteDirectoryView => initialView;
  let view = $state.raw(readInitialView());
  let query = $state.raw<SiteDirectoryQuery>(readInitialView().query);
  let searchDraft = $state(readInitialView().query.q);
  let pending = $state(false);
  let failed = $state(false);
  let statusMessage = $state('');
  let failedQuery = $state.raw<SiteDirectoryQuery | undefined>();
  let requestController: AbortController | undefined;
  let requestSequence = 0;

  onDestroy(() => requestController?.abort());

  async function load(
    nextQuery: SiteDirectoryQuery,
    cause: LoadCause,
    synchronizeURL = true,
  ): Promise<void> {
    requestController?.abort();
    const controller = new AbortController();
    const sequence = ++requestSequence;
    requestController = controller;
    query = nextQuery;
    pending = true;
    failed = false;
    statusMessage = '正在更新博客列表';
    try {
      const nextView = await refreshSiteDirectory(nextQuery, controller.signal);
      if (sequence !== requestSequence) return;
      view = nextView;
      query = nextView.query;
      failedQuery = undefined;
      if (cause === 'search' || cause === 'popstate') searchDraft = nextView.query.q;
      statusMessage = `已显示 ${nextView.pagination.totalItems} 个博客中的第 ${nextView.pagination.page} 页`;
      if (synchronizeURL) synchronizeQueryURL(nextView.query);
      if (cause === 'page') scrollToResults();
    } catch (error) {
      if (
        sequence !== requestSequence ||
        (error instanceof DOMException && error.name === 'AbortError')
      )
        return;
      if (!(error instanceof Error)) throw error;
      failedQuery = nextQuery;
      query = view.query;
      failed = true;
      statusMessage = '博客列表更新失败';
    } finally {
      if (sequence === requestSequence) {
        pending = false;
        requestController = undefined;
      }
    }
  }

  function synchronizeQueryURL(nextQuery: SiteDirectoryQuery): void {
    const parameters = buildSiteDirectorySearchParams(nextQuery);
    window.history.replaceState(null, '', `${window.location.pathname}?${parameters.toString()}`);
  }

  function scrollToResults(): void {
    document.querySelector('#directory-results-title')?.scrollIntoView({
      behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
      block: 'start',
    });
  }

  function updateQuery(patch: Partial<SiteDirectoryQuery>, cause: LoadCause): void {
    void load(
      { ...query, ...patch, page: cause === 'page' ? (patch.page ?? query.page) : 1 },
      cause,
    );
  }

  function submitSearch(): void {
    const nextSearch = searchDraft.trim();
    if (nextSearch !== query.q) updateQuery({ q: nextSearch }, 'search');
  }

  function toggleValues(
    values: readonly string[],
    value: string,
    selected: boolean,
  ): readonly string[] {
    return selected ? [...new Set([...values, value])] : values.filter((item) => item !== value);
  }

  function handleFilterToggle(
    name: SiteDirectoryFilterName,
    value: string,
    selected: boolean,
  ): void {
    switch (name) {
      case 'primary':
        updateQuery({ primary: toggleValues(query.primary, value, selected) }, 'filter');
        break;
      case 'secondary':
        updateQuery({ secondary: toggleValues(query.secondary, value, selected) }, 'filter');
        break;
      case 'warning':
        updateQuery({ warning: toggleValues(query.warning, value, selected) }, 'filter');
        break;
      case 'technology':
        updateQuery({ technology: toggleValues(query.technology, value, selected) }, 'filter');
        break;
      case 'access':
        updateQuery({ access: toggleAccess(query.access, value, selected) }, 'filter');
        break;
    }
  }

  function toggleAccess(
    values: readonly SiteDirectoryAccess[],
    value: string,
    selected: boolean,
  ): readonly SiteDirectoryAccess[] {
    return toggleValues(values, value, selected).filter(
      (item): item is SiteDirectoryAccess =>
        item === 'ALL' || item === 'CN_ONLY' || item === 'GLOBAL_ONLY',
    );
  }

  function removeFilter(name: SiteDirectoryFilterName | 'feed', value: string): void {
    if (name === 'feed') {
      updateQuery({ feed: 'any' }, 'filter');
      return;
    }
    handleFilterToggle(name, value, false);
  }

  function clearFilters(): void {
    updateQuery(
      { primary: [], secondary: [], warning: [], technology: [], access: [], feed: 'any' },
      'filter',
    );
  }

  function handlePopState(): void {
    const restoredQuery = parseSiteDirectorySearchParams(
      new URL(window.location.href).searchParams,
    );
    searchDraft = restoredQuery.q;
    void load(restoredQuery, 'popstate', false);
  }
</script>

<svelte:window onpopstate={handlePopState} />

<SiteDirectoryHero
  {searchDraft}
  {pending}
  onSearchInput={(value) => (searchDraft = value)}
  onSearchSubmit={submitSearch}
/>

<main
  class="mx-auto grid w-[min(calc(100%-2rem),80rem)] gap-6 py-6 sm:w-[min(calc(100%-3rem),80rem)] lg:grid-cols-[16rem_minmax(0,1fr)] lg:items-start lg:gap-8 lg:py-10"
>
  <SiteDirectoryFilterShell
    {options}
    {query}
    onToggle={handleFilterToggle}
    onFeedChange={(feed: SiteDirectoryFeed) => updateQuery({ feed }, 'filter')}
  />

  <div class="min-w-0">
    <SiteDirectoryToolbar
      {view}
      {pending}
      onStatusChange={(status: SiteDirectoryStatus) => updateQuery({ status }, 'status')}
      onSortChange={(sort: SiteDirectorySort) => updateQuery({ sort }, 'sort')}
      onOrderChange={(order: SiteDirectoryOrder) => updateQuery({ order }, 'sort')}
      onShuffle={() =>
        updateQuery({ seed: createDirectoryShuffleSeed(), sort: 'random' }, 'shuffle')}
    />
    <SiteDirectorySelectedFilters
      {options}
      {query}
      onRemove={removeFilter}
      onClear={clearFilters}
    />

    {#if failed}
      <div
        class="my-5 flex flex-wrap items-center justify-between gap-3 rounded-md border border-l-3 border-line border-l-danger bg-danger-bg p-4 text-sm text-danger-fg"
        role="alert"
      >
        <span>列表更新失败，当前结果已保留。</span>
        <button
          class="inline-flex min-h-11 items-center gap-1.5 rounded-md px-3 font-medium hover:bg-danger-fg/10 sm:min-h-10"
          type="button"
          onclick={() => void load(failedQuery ?? query, 'retry')}
        >
          <IconRefresh aria-hidden="true" size={16} stroke={1.8} />
          重试
        </button>
      </div>
    {/if}

    {#key view}
      <SiteDirectoryResults
        {view}
        {pending}
        onPageChange={(page) => updateQuery({ page }, 'page')}
      />
    {/key}
  </div>
</main>

<p class="sr-only" aria-live="polite" aria-atomic="true">{statusMessage}</p>

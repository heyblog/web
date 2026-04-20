<script lang="ts">
  import { onMount, untrack } from 'svelte';

  import type {
    SiteDirectoryItem,
    SiteDirectoryMeta,
    SiteDirectoryPreference,
    SiteDirectoryResult,
  } from '@/application/site/site-directory.models';
  import { parseSiteDirectoryStructuredSearch } from '@/application/site/site-directory.search';
  import {
    applyDirectoryPreference,
    hasExplicitDirectoryPreference,
    type SiteDirectoryQueryState,
  } from '@/application/site/site-directory.shared';
  import { submitPublicSiteFeedback } from '@/application/site/site-feedback.browser';
  import {
    persistSiteDirectoryPreference,
    readSiteDirectoryPreference,
    requestSiteDirectory,
    requestSiteDirectoryMeta,
    resolveSiteDirectoryAccessSummary,
    resolveSiteDirectorySortSummary,
    syncSiteDirectoryUrl,
  } from '@/components/site/site-directory-page.api';
  import {
    appendSiteDirectorySyntaxSnippet,
    changeSiteDirectoryPage,
    handleSiteDirectoryBooleanFilter,
    handleSiteDirectoryOrderToggle,
    handleSiteDirectoryRandomToggle,
    handleSiteDirectorySearchClear,
    handleSiteDirectorySearchDrivenFilter,
    handleSiteDirectorySearchSubmit,
    handleSiteDirectorySortChange,
    handleSiteDirectoryStatusModeChange,
  } from '@/components/site/site-directory-page.logic';
  import {
    cloneSiteDirectoryMeta,
    cloneSiteDirectoryPreference,
    cloneSiteDirectoryResult,
    createInitialSiteDirectoryQuery,
  } from '@/components/site/site-directory-page.shared';
  import SiteDirectoryResultsSection from '@/components/site/SiteDirectoryResultsSection.svelte';
  import SiteDirectorySearchSection from '@/components/site/SiteDirectorySearchSection.svelte';
  import SiteFeedbackDialog from '@/components/site/SiteFeedbackDialog.svelte';

  let {
    initialMeta,
    initialResult,
    initialPreference = null,
    canUsePreference = false,
  }: {
    initialMeta: SiteDirectoryMeta;
    initialResult: SiteDirectoryResult;
    initialPreference?: SiteDirectoryPreference | null;
    canUsePreference?: boolean;
  } = $props();

  const initialMetaValue = untrack(() => cloneSiteDirectoryMeta(initialMeta));
  const initialResultValue = untrack(() => cloneSiteDirectoryResult(initialResult));
  const initialPreferenceValue = untrack(() => cloneSiteDirectoryPreference(initialPreference));

  let meta = $state(initialMetaValue);
  let result = $state(initialResultValue);
  let query = $state<SiteDirectoryQueryState>(createInitialSiteDirectoryQuery(initialResultValue));
  let draftSearch = $state(initialResultValue.query.q);
  let pending = $state(false);
  let feedbackTarget = $state<SiteDirectoryItem | null>(null);
  let feedbackSubmitting = $state(false);
  let preference = $state<SiteDirectoryPreference | null>(initialPreferenceValue);
  let syntaxHelpOpen = $state(false);
  let metaLoading = $state(false);

  const draftStructured = $derived(parseSiteDirectoryStructuredSearch(draftSearch));
  const sortSummary = $derived(resolveSiteDirectorySortSummary(query.sort));
  const accessSummary = $derived(resolveSiteDirectoryAccessSummary(draftStructured.access[0]));
  const featuredSummary = $derived(
    draftStructured.featured === null ? '' : draftStructured.featured ? '已推荐' : '未推荐',
  );
  const hasLoadedFilters = $derived(
    meta.filters.mainTags.length > 0 ||
      meta.filters.subTags.length > 0 ||
      meta.filters.warningTags.length > 0 ||
      meta.filters.programs.length > 0,
  );

  async function loadMetaFilters() {
    if (metaLoading || hasLoadedFilters) {
      return;
    }

    metaLoading = true;

    try {
      const nextMeta = await requestSiteDirectoryMeta();

      if (!nextMeta) {
        return;
      }

      meta = cloneSiteDirectoryMeta(nextMeta);
    } finally {
      metaLoading = false;
    }
  }

  function scheduleMetaFiltersLoad() {
    if (typeof window === 'undefined' || hasLoadedFilters) {
      return;
    }

    if ('requestIdleCallback' in window) {
      window.requestIdleCallback(() => {
        void loadMetaFilters();
      });
      return;
    }

    globalThis.setTimeout(() => {
      void loadMetaFilters();
    }, 160);
  }

  async function loadDirectory(
    nextQuery: SiteDirectoryQueryState,
    options: { persistRandomPreference?: boolean } = {},
  ) {
    pending = true;

    try {
      const nextResult = await requestSiteDirectory(nextQuery);

      if (!nextResult) {
        return;
      }

      result = nextResult;
      query = {
        ...nextResult.query,
        page: nextResult.pagination.page,
        pageSize: nextResult.pagination.pageSize,
      };
      draftSearch = nextResult.query.q;
      syncSiteDirectoryUrl({
        ...nextResult.query,
        page: nextResult.pagination.page,
        pageSize: nextResult.pagination.pageSize,
      });

      if (options.persistRandomPreference) {
        const nextPreference: SiteDirectoryPreference = {
          randomMode: nextQuery.random ? 'stable' : 'off',
          randomSeed: nextQuery.random ? nextQuery.randomSeed : nextQuery.randomSeed || null,
        };
        preference = nextPreference;
        await persistSiteDirectoryPreference(canUsePreference, nextPreference);
      }
    } finally {
      pending = false;
    }
  }

  const searchContext = $derived({
    query,
    draftSearch,
    structured: draftStructured,
    setDraftSearch: (value: string) => {
      draftSearch = value;
    },
    loadDirectory,
  });

  async function submitFeedback(payload: {
    reasonType: string;
    feedbackContent: string;
    reporterName?: string | null;
    reporterEmail?: string | null;
    notifyByEmail?: boolean;
  }) {
    if (!feedbackTarget) {
      return;
    }

    feedbackSubmitting = true;

    try {
      const ok = await submitPublicSiteFeedback(feedbackTarget.slug, payload);

      if (ok) {
        feedbackTarget = null;
      }
    } finally {
      feedbackSubmitting = false;
    }
  }

  onMount(async () => {
    if (typeof window === 'undefined') {
      return;
    }

    scheduleMetaFiltersLoad();

    if (!canUsePreference) {
      return;
    }

    const hasExplicitPreference = hasExplicitDirectoryPreference(
      new URLSearchParams(window.location.search),
    );

    if (hasExplicitPreference) {
      return;
    }

    const nextPreference = preference ?? (await readSiteDirectoryPreference(canUsePreference));

    if (!nextPreference) {
      return;
    }

    preference = nextPreference;
    const preferredQuery = applyDirectoryPreference(query, nextPreference);

    if (
      preferredQuery.random === query.random &&
      preferredQuery.randomSeed === query.randomSeed &&
      preferredQuery.sort === query.sort
    ) {
      return;
    }

    await loadDirectory(preferredQuery);
  });
</script>

<div class="page-stack">
  <SiteDirectorySearchSection
    {meta}
    value={draftSearch}
    {pending}
    structured={draftStructured}
    {syntaxHelpOpen}
    {accessSummary}
    {featuredSummary}
    onSearchChange={(value) => {
      draftSearch = value;
    }}
    onSearchSubmit={() => handleSiteDirectorySearchSubmit(searchContext)}
    onSearchClear={() => handleSiteDirectorySearchClear(searchContext)}
    onSyntaxToggle={() => {
      syntaxHelpOpen = !syntaxHelpOpen;
    }}
    onInsertSyntaxSnippet={(snippet) =>
      appendSiteDirectorySyntaxSnippet(
        draftSearch,
        (value) => {
          draftSearch = value;
        },
        snippet,
      )}
    onSearchDrivenFilter={(field, value, multiple) =>
      handleSiteDirectorySearchDrivenFilter(searchContext, field, value, multiple)}
    onBooleanFilter={(field, value) =>
      handleSiteDirectoryBooleanFilter(searchContext, field, value)}
  />

  <SiteDirectoryResultsSection
    {result}
    {pending}
    {sortSummary}
    onStatusModeChange={(nextStatusMode) =>
      handleSiteDirectoryStatusModeChange(searchContext, nextStatusMode)}
    onRandomToggle={() => handleSiteDirectoryRandomToggle(searchContext)}
    onSortChange={(value) => handleSiteDirectorySortChange(searchContext, value)}
    onOrderToggle={() => handleSiteDirectoryOrderToggle(searchContext)}
    onChangePage={(nextPage) => changeSiteDirectoryPage(result, searchContext, nextPage)}
    onFeedback={(item) => {
      feedbackTarget = item;
    }}
  />
</div>

<SiteFeedbackDialog
  open={Boolean(feedbackTarget)}
  siteName={feedbackTarget?.name ?? '站点'}
  submitting={feedbackSubmitting}
  onCancel={() => {
    feedbackTarget = null;
  }}
  onSubmit={submitFeedback}
/>

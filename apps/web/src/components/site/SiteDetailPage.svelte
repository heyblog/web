<script lang="ts">
  import { onMount, untrack } from 'svelte';

  import { resolveTone } from '@/application/site/site-card.shared';
  import type {
    PagedResult,
    SiteArticleItem,
    SiteCheckItem,
    SiteDetail,
  } from '@/application/site/site-directory.models';
  import { submitPublicSiteFeedback } from '@/application/site/site-feedback.browser';
  import {
    buildHeartbeatChecks,
    buildSiteResourceLinks,
    cloneSiteDetailPagedResult,
    resolveHeartbeatSlotCount,
    resolveSiteStatusToneClass,
  } from '@/components/site/site-detail.shared';
  import SiteDetailHeroSection from '@/components/site/SiteDetailHeroSection.svelte';
  import SiteDetailSidebar from '@/components/site/SiteDetailSidebar.svelte';
  import SiteDetailTabs from '@/components/site/SiteDetailTabs.svelte';
  import SiteFeedbackDialog from '@/components/site/SiteFeedbackDialog.svelte';

  let {
    detail,
    initialArticles,
    initialChecks,
  }: {
    detail: SiteDetail;
    initialArticles: PagedResult<SiteArticleItem>;
    initialChecks: PagedResult<SiteCheckItem>;
  } = $props();

  const initialArticlesValue = untrack(() => cloneSiteDetailPagedResult(initialArticles));
  const initialChecksValue = untrack(() => cloneSiteDetailPagedResult(initialChecks));

  let activeTab = $state<'articles' | 'checks' | 'history'>('articles');
  let articles = $state(initialArticlesValue);
  let checks = $state(initialChecksValue);
  let loadingTab = $state<'articles' | 'checks' | null>(null);
  let copiedKey = $state('');
  let feedbackOpen = $state(false);
  let feedbackSubmitting = $state(false);
  let viewportWidth = $state(1440);

  const tagTone = $derived(resolveTone(detail.primaryTag ?? '未分类', detail.featured));
  const statusToneClass = $derived(resolveSiteStatusToneClass(detail.status));
  const heartbeatSlotCount = $derived(resolveHeartbeatSlotCount(viewportWidth));
  const heartbeatChecks = $derived(
    buildHeartbeatChecks(detail.heartbeatChecks, heartbeatSlotCount),
  );
  const resourceLinks = $derived(buildSiteResourceLinks(detail));

  onMount(() => {
    const updateViewportWidth = () => {
      viewportWidth = window.innerWidth;
    };

    updateViewportWidth();
    window.addEventListener('resize', updateViewportWidth);

    return () => {
      window.removeEventListener('resize', updateViewportWidth);
    };
  });

  async function copyValue(key: string, value: string | null) {
    if (!value) {
      return;
    }

    try {
      await navigator.clipboard.writeText(value);
      copiedKey = key;

      window.setTimeout(() => {
        if (copiedKey === key) {
          copiedKey = '';
        }
      }, 1600);
    } catch {
      copiedKey = '';
    }
  }

  async function loadTabPage(tab: 'articles' | 'checks', page: number) {
    loadingTab = tab;

    try {
      const response = await fetch(
        tab === 'articles'
          ? `/api/site-directory/${detail.slug}/articles?page=${page}&pageSize=${articles.pagination.pageSize}`
          : `/api/site-directory/${detail.slug}/checks?page=${page}&pageSize=${checks.pagination.pageSize}`,
        {
          headers: { accept: 'application/json' },
        },
      );

      if (!response.ok) {
        return;
      }

      if (tab === 'articles') {
        const payload = (await response.json()) as {
          ok: boolean;
          data: PagedResult<SiteArticleItem>;
        };
        articles = payload.data;
      } else {
        const payload = (await response.json()) as {
          ok: boolean;
          data: PagedResult<SiteCheckItem>;
        };
        checks = payload.data;
      }
    } finally {
      loadingTab = null;
    }
  }

  async function submitFeedback(payload: {
    reasonType: string;
    feedbackContent: string;
    reporterName?: string | null;
    reporterEmail?: string | null;
    notifyByEmail?: boolean;
  }) {
    feedbackSubmitting = true;

    try {
      const ok = await submitPublicSiteFeedback(detail.slug, payload);

      if (ok) {
        feedbackOpen = false;
      }
    } finally {
      feedbackSubmitting = false;
    }
  }
</script>

<div class="page-stack">
  <section class="page-section pt-2">
    <div class="grid gap-10 xl:grid-cols-[minmax(0,1.12fr)_22rem]">
      <div class="flex flex-col gap-5">
        <SiteDetailHeroSection
          {detail}
          {tagTone}
          {statusToneClass}
          {heartbeatChecks}
          onOpenFeedback={() => {
            feedbackOpen = true;
          }}
        />
        <SiteDetailTabs
          siteId={detail.id}
          {activeTab}
          {articles}
          {checks}
          {loadingTab}
          onTabChange={(tab) => {
            activeTab = tab;
          }}
          onLoadPage={loadTabPage}
        />
      </div>
      <SiteDetailSidebar {detail} {resourceLinks} {copiedKey} onCopy={copyValue} />
    </div>
  </section>
</div>

<SiteFeedbackDialog
  open={feedbackOpen}
  siteName={detail.name}
  submitting={feedbackSubmitting}
  onCancel={() => {
    feedbackOpen = false;
  }}
  onSubmit={submitFeedback}
/>

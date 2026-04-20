<script lang="ts">
  import {
    createEmptyFeedDraft,
    FEED_TYPE_OPTIONS,
    type FeedTypeKey,
    type SiteSnapshotDraft,
  } from '@/application/management/site-management.snapshot';
  import {
    ensureSingleDefaultFeedSelection,
    formatFeedIndex,
    shouldPromptDefaultFeedSelection,
  } from '@/application/site-submission/site-feed-draft';
  import {
    WORKSPACE_INPUT_CLASS,
    WORKSPACE_SELECT_CHEVRON_STYLE,
    WORKSPACE_SELECT_CLASS,
  } from '@/components/site-submission/site-submission-workspace.constants';

  let {
    draft = $bindable(),
    disabled = false,
    idPrefix = 'site-management-fields',
    fieldAlerts = {},
  }: {
    draft: SiteSnapshotDraft;
    disabled?: boolean;
    idPrefix?: string;
    fieldAlerts?: Partial<Record<string, { label: string; value: string }>>;
  } = $props();

  type DraftTextField = 'sitemap' | 'link_page';
  type FeedDraftPatch = Partial<SiteSnapshotDraft['feeds'][number]>;

  const updateDraft = (patch: Partial<SiteSnapshotDraft>): void => {
    draft = {
      ...draft,
      ...patch,
    };
  };

  const updateDraftTextField = (field: DraftTextField, value: string): void => {
    draft[field] = value;
    draft = { ...draft };
  };

  const replaceFeeds = (feeds: SiteSnapshotDraft['feeds'], selectedId?: string | null): void => {
    updateDraft({
      feeds: ensureSingleDefaultFeedSelection(feeds, selectedId),
    });
  };

  const patchFeed = (feedId: string, patch: FeedDraftPatch, selectedId?: string | null): void => {
    replaceFeeds(
      draft.feeds.map((item) => (item.id === feedId ? { ...item, ...patch } : item)),
      selectedId,
    );
  };

  const handleAddFeed = (): void => {
    updateDraft({
      feeds: ensureSingleDefaultFeedSelection([...draft.feeds, createEmptyFeedDraft()]),
    });
  };

  const handleRemoveFeed = (feedId: string): void => {
    const nextFeeds = draft.feeds.filter((item) => item.id !== feedId);
    const fallbackId =
      draft.feeds.find((item) => item.id !== feedId && item.isDefault)?.id ??
      nextFeeds[0]?.id ??
      null;

    if (nextFeeds.length === 0) {
      updateDraft({
        feeds: [],
      });
      return;
    }

    replaceFeeds(nextFeeds, fallbackId);
  };

  const handleFeedNameInput = (feedId: string, value: string): void => {
    patchFeed(feedId, { name: value });
  };

  const handleFeedUrlInput = (feedId: string, value: string): void => {
    const current = draft.feeds.find((item) => item.id === feedId);
    patchFeed(feedId, { url: value }, current?.isDefault ? current.id : undefined);
  };

  const handleFeedTypeInput = (feedId: string, value: FeedTypeKey): void => {
    patchFeed(feedId, { type: value });
  };

  const handleSelectDefaultFeed = (feedId: string): void => {
    replaceFeeds(draft.feeds, feedId);
  };

  let canChooseDefaultFeed = $derived(draft.feeds.length > 1);
  let feedMetaCopy = $derived(
    draft.feeds.length === 1
      ? '单个订阅时可留空名称，保存时会自动记为默认订阅。'
      : '多个订阅时请明确填写每个订阅名称。',
  );
</script>

<div class="space-y-4">
  <div class="subscription-feed-section-header border-t border-(--color-line) pt-5">
    <span class="subscription-feed-section-label">订阅地址</span>
    {#if draft.feeds.length > 0}
      <span class="subscription-feed-count-pill">{draft.feeds.length}</span>
    {/if}
  </div>

  {#if fieldAlerts.feed}
    <div
      class="rounded-sm border border-[color-mix(in_srgb,var(--color-fail)_32%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-fail)_8%,transparent)] px-3 py-2"
    >
      <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改前</p>
      <p class="mt-1 text-xs whitespace-pre-wrap text-(--color-fg)">{fieldAlerts.feed.value}</p>
    </div>
  {/if}

  {#if draft.feeds.length > 0}
    <div class="subscription-feed-list">
      {#each draft.feeds as feed, index (feed.id)}
        <article class={`subscription-feed-card ${feed.isDefault ? 'is-default' : ''}`}>
          <div class="subscription-feed-card-header">
            <span class="subscription-feed-card-index">{formatFeedIndex(index)}</span>
            <span class="subscription-feed-spacer"></span>
            {#if canChooseDefaultFeed}
              <button
                class={`subscription-feed-default-toggle ${feed.isDefault ? 'active' : ''}`}
                type="button"
                onclick={() => handleSelectDefaultFeed(feed.id)}
                {disabled}
                aria-label={`设为默认订阅 ${index + 1}`}
              >
                <span>{feed.isDefault ? '默认订阅' : '设为默认'}</span>
              </button>
            {/if}
            <button
              class="subscription-feed-delete-button"
              type="button"
              onclick={() => handleRemoveFeed(feed.id)}
              {disabled}
              aria-label={`删除订阅 ${index + 1}`}
              title="删除"
            >
              ×
            </button>
          </div>

          <div class="subscription-feed-card-body">
            <div class="subscription-feed-fields with-type">
              <label class="subscription-feed-field">
                <span>订阅名称</span>
                <input
                  class={WORKSPACE_INPUT_CLASS}
                  {disabled}
                  value={feed.name}
                  oninput={(event) =>
                    handleFeedNameInput(feed.id, (event.currentTarget as HTMLInputElement).value)}
                  placeholder={draft.feeds.length === 1 ? '如：主站更新' : `订阅名称 ${index + 1}`}
                />
              </label>

              <label class="subscription-feed-field">
                <span>订阅链接</span>
                <input
                  class={WORKSPACE_INPUT_CLASS}
                  {disabled}
                  value={feed.url}
                  oninput={(event) =>
                    handleFeedUrlInput(feed.id, (event.currentTarget as HTMLInputElement).value)}
                  placeholder="https://example.com/feed"
                />
              </label>

              <label class="subscription-feed-field">
                <span>格式类型</span>
                <select
                  class={WORKSPACE_SELECT_CLASS}
                  style={WORKSPACE_SELECT_CHEVRON_STYLE}
                  {disabled}
                  value={feed.type ?? 'RSS'}
                  onchange={(event) =>
                    handleFeedTypeInput(
                      feed.id,
                      (event.currentTarget as HTMLSelectElement).value as FeedTypeKey,
                    )}
                >
                  {#each FEED_TYPE_OPTIONS as item (item.value)}
                    <option value={item.value}>{item.label}</option>
                  {/each}
                </select>
              </label>
            </div>

            <div class="subscription-feed-meta">
              <span>{feedMetaCopy}</span>
            </div>
          </div>
        </article>
      {/each}
    </div>
  {/if}

  <button class="subscription-feed-add-button" type="button" onclick={handleAddFeed} {disabled}>
    <span class="subscription-feed-add-icon">+</span>
    添加订阅地址
  </button>

  {#if shouldPromptDefaultFeedSelection(draft.feeds)}
    <p class="text-xs text-(--color-fail)">请选择一个默认订阅地址用于本站订阅抓取</p>
  {/if}

  <div class="grid gap-4 md:grid-cols-2">
    <div class="space-y-2">
      {#if fieldAlerts.sitemap}
        <div
          class="rounded-sm border border-[color-mix(in_srgb,var(--color-fail)_32%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-fail)_8%,transparent)] px-3 py-2"
        >
          <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改前</p>
          <p class="mt-1 text-xs whitespace-pre-wrap text-(--color-fg)">
            {fieldAlerts.sitemap.value}
          </p>
        </div>
      {/if}
      <label class="block text-sm" for={`${idPrefix}-sitemap`}>网站地图</label>
      <input
        id={`${idPrefix}-sitemap`}
        class={WORKSPACE_INPUT_CLASS}
        {disabled}
        value={draft.sitemap}
        oninput={(event) =>
          updateDraftTextField('sitemap', (event.currentTarget as HTMLInputElement).value)}
        placeholder="https://example.com/sitemap.xml"
      />
    </div>

    <div class="space-y-2">
      {#if fieldAlerts.link_page}
        <div
          class="rounded-sm border border-[color-mix(in_srgb,var(--color-fail)_32%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-fail)_8%,transparent)] px-3 py-2"
        >
          <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改前</p>
          <p class="mt-1 text-xs whitespace-pre-wrap text-(--color-fg)">
            {fieldAlerts.link_page.value}
          </p>
        </div>
      {/if}
      <label class="block text-sm" for={`${idPrefix}-link-page`}>友链页面</label>
      <input
        id={`${idPrefix}-link-page`}
        class={WORKSPACE_INPUT_CLASS}
        {disabled}
        value={draft.link_page}
        oninput={(event) =>
          updateDraftTextField('link_page', (event.currentTarget as HTMLInputElement).value)}
        placeholder="https://example.com/friends"
      />
    </div>
  </div>
</div>

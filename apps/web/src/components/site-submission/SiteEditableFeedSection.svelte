<script lang="ts">
  import {
    formatFeedIndex,
    shouldPromptDefaultFeedSelection,
  } from '@/application/site-submission/site-feed-draft';
  import type { AutoFillFieldKey } from '@/application/site-submission/site-submission.browser-workspace';
  import type {
    FeedType,
    FieldErrors,
  } from '@/application/site-submission/site-submission.service';

  import type { CommonSiteForm, FeedTypeOption } from './site-editable-fields.types';

  let {
    form = $bindable(),
    errors = {},
    disabled = false,
    idPrefix = 'site-fields',
    inputClass = '',
    selectClass = '',
    selectChevronStyle = '',
    allowFeedTypeSelect = false,
    feedTypeOptions = [],
    withInputStateClass = (base) => base,
    fieldNeedsRefinement = () => false,
    isAutoFillMissing = () => false,
    clearAutoFillMissing = () => {},
    addFeed = undefined,
    removeFeed = undefined,
    updateFeedName = undefined,
    updateFeedUrl = undefined,
    selectDefaultFeed = undefined,
    updateFeedType = undefined,
  }: {
    form: CommonSiteForm;
    errors?: FieldErrors;
    disabled?: boolean;
    idPrefix?: string;
    inputClass?: string;
    selectClass?: string;
    selectChevronStyle?: string;
    allowFeedTypeSelect?: boolean;
    feedTypeOptions?: FeedTypeOption[];
    withInputStateClass?: (base: string, warned: boolean, missing: boolean) => string;
    fieldNeedsRefinement?: (value: string) => boolean;
    isAutoFillMissing?: (field: AutoFillFieldKey) => boolean;
    clearAutoFillMissing?: (field: AutoFillFieldKey) => void;
    addFeed?: (() => void) | undefined;
    removeFeed?: ((id: string) => void) | undefined;
    updateFeedName?: ((id: string, value: string) => void) | undefined;
    updateFeedUrl?: ((id: string, value: string) => void) | undefined;
    selectDefaultFeed?: ((id: string) => void) | undefined;
    updateFeedType?: ((id: string, value: FeedType) => void) | undefined;
  } = $props();

  const syncForm = (): void => {
    form = { ...form };
  };

  const handleAddFeed = (): void => {
    addFeed?.();
  };

  const handleRemoveFeed = (feedId: string): void => {
    removeFeed?.(feedId);
  };

  const handleFeedNameInput = (feedId: string, value: string): void => {
    updateFeedName?.(feedId, value);
  };

  const handleFeedUrlInput = (feedId: string, value: string): void => {
    updateFeedUrl?.(feedId, value);
    clearAutoFillMissing('feeds');
    syncForm();
  };

  const handleFeedTypeInput = (feedId: string, value: FeedType): void => {
    updateFeedType?.(feedId, value);
  };

  const handleSelectDefaultFeed = (feedId: string): void => {
    selectDefaultFeed?.(feedId);
  };
</script>

<div class="space-y-4">
  <div class="subscription-feed-section-header border-t border-(--color-line) pt-5">
    <span class="subscription-feed-section-label">订阅地址</span>
    {#if form.feeds.length > 0}
      <span class="subscription-feed-count-pill">{form.feeds.length}</span>
    {/if}
  </div>

  {#if form.feeds.length > 0}
    <div class="subscription-feed-list">
      {#each form.feeds as feed, index (feed.id)}
        <article class={`subscription-feed-card ${feed.isDefault ? 'is-default' : ''}`}>
          <div class="subscription-feed-card-header">
            <span class="subscription-feed-card-index">{formatFeedIndex(index)}</span>
            <span class="subscription-feed-spacer"></span>
            {#if form.feeds.length > 1}
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
            <div
              class={`subscription-feed-fields ${allowFeedTypeSelect ? 'with-type' : 'without-type'}`}
            >
              <label class="subscription-feed-field">
                <span>订阅名称</span>
                <input
                  class={inputClass}
                  {disabled}
                  value={feed.name}
                  oninput={(event) =>
                    handleFeedNameInput(feed.id, (event.currentTarget as HTMLInputElement).value)}
                  placeholder={form.feeds.length === 1 ? '如：主站更新' : `订阅名称 ${index + 1}`}
                />
              </label>

              <label class="subscription-feed-field">
                <span>订阅链接</span>
                <input
                  class={withInputStateClass(
                    inputClass,
                    fieldNeedsRefinement(feed.url),
                    isAutoFillMissing('feeds'),
                  )}
                  {disabled}
                  value={feed.url}
                  oninput={(event) =>
                    handleFeedUrlInput(feed.id, (event.currentTarget as HTMLInputElement).value)}
                  placeholder="https://example.com/feed"
                />
              </label>

              {#if allowFeedTypeSelect}
                <label class="subscription-feed-field">
                  <span>格式类型</span>
                  <select
                    class={selectClass}
                    style={selectChevronStyle}
                    {disabled}
                    value={feed.type ?? 'RSS'}
                    onchange={(event) =>
                      handleFeedTypeInput(
                        feed.id,
                        (event.currentTarget as HTMLSelectElement).value as FeedType,
                      )}
                  >
                    {#each feedTypeOptions as item (item.value)}
                      <option value={item.value}>{item.label}</option>
                    {/each}
                  </select>
                </label>
              {/if}
            </div>

            <div class="subscription-feed-meta">
              <span>
                {form.feeds.length === 1
                  ? '单个订阅时可留空名称，提交时会自动记为默认订阅。'
                  : '多个订阅时请明确填写每个订阅名称。'}
              </span>
            </div>

            {#if fieldNeedsRefinement(feed.url) && !errors.feeds}
              <p class="text-xs text-(--color-fail)">
                当前订阅地址与站点地址一致，请补充具体路径或清空该项。
              </p>
            {/if}
          </div>
        </article>
      {/each}
    </div>
  {/if}

  <button class="subscription-feed-add-button" type="button" onclick={handleAddFeed} {disabled}>
    <span class="subscription-feed-add-icon">+</span>
    添加订阅地址
  </button>

  {#if shouldPromptDefaultFeedSelection(form.feeds) && !errors.feeds}
    <p class="text-xs text-(--color-fail)">请选择一个默认订阅地址用于本站订阅抓取</p>
  {/if}
  {#if errors.feeds}<p class="text-xs text-(--color-fail)">{errors.feeds}</p>{/if}

  <div class="grid gap-4 md:grid-cols-2">
    <div class="space-y-2">
      <label class="block text-sm" for={`${idPrefix}-sitemap`}>网站地图</label>
      <input
        id={`${idPrefix}-sitemap`}
        class={withInputStateClass(
          inputClass,
          fieldNeedsRefinement(form.sitemap),
          isAutoFillMissing('sitemap'),
        )}
        {disabled}
        value={form.sitemap}
        oninput={(event) => {
          form.sitemap = (event.currentTarget as HTMLInputElement).value;
          clearAutoFillMissing('sitemap');
          syncForm();
        }}
        placeholder="https://example.com/sitemap.xml"
      />
      {#if errors.sitemap}<p class="text-xs text-(--color-fail)">{errors.sitemap}</p>{/if}
      {#if fieldNeedsRefinement(form.sitemap) && !errors.sitemap}
        <p class="text-xs text-(--color-fail)">网站地图与站点地址相同，请补充路径或清空该字段。</p>
      {/if}
    </div>

    <div class="space-y-2">
      <label class="block text-sm" for={`${idPrefix}-link-page`}>友链页面</label>
      <input
        id={`${idPrefix}-link-page`}
        class={withInputStateClass(
          inputClass,
          fieldNeedsRefinement(form.link_page),
          isAutoFillMissing('linkPage'),
        )}
        {disabled}
        value={form.link_page}
        oninput={(event) => {
          form.link_page = (event.currentTarget as HTMLInputElement).value;
          clearAutoFillMissing('linkPage');
          syncForm();
        }}
        placeholder="https://example.com/friends"
      />
      {#if errors.link_page}<p class="text-xs text-(--color-fail)">{errors.link_page}</p>{/if}
      {#if fieldNeedsRefinement(form.link_page) && !errors.link_page}
        <p class="text-xs text-(--color-fail)">友链页与站点地址相同，请补充路径或清空该字段。</p>
      {/if}
    </div>
  </div>
</div>

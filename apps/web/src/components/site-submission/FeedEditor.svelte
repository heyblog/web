<script lang="ts">
  import { IconPlus, IconTrash } from '@tabler/icons-svelte';

  import {
    addFeed,
    type EditableSubmission,
    removeFeed,
    setDefaultFeed,
  } from '@/application/site-submission/site-submission.browser';
  import { feedFormats } from '@/application/site-submission/site-submission.types';
  import { validateAuxiliaryURLs } from '@/application/site-submission/site-submission.validation';
  interface Props {
    form: EditableSubmission;
  }
  let { form = $bindable() }: Props = $props();
  let urlValidation = $derived(validateAuxiliaryURLs(form));
</script>

<section class="grid min-w-0 gap-4">
  <header class="flex min-w-0 flex-wrap items-start justify-between gap-4">
    <div class="min-w-0">
      <h2 class="text-xl font-semibold">Feed 与站点资源</h2>
      <p class="mt-1 text-sm text-fg-muted">
        <span class="block">请填写真实资源地址；没有时可以留空。</span>
      </p>
    </div>
    <button
      class="inline-flex min-h-11 items-center gap-2 rounded-sm border border-line-strong px-3 text-sm font-medium disabled:opacity-50"
      type="button"
      disabled={form.feeds.length >= 8}
      onclick={() => addFeed(form)}><IconPlus size={18} aria-hidden="true" />添加 Feed</button
    >
  </header>
  {#if form.feeds.length === 0}<p
      class="rounded-sm border border-dashed border-line p-4 text-sm text-fg-muted"
    >
      还没有添加 Feed。点击右上角的“添加 Feed”按钮来添加一个。
    </p>{/if}
  {#each form.feeds as feed (feed.id)}
    {@const feedMessage = urlValidation.feedMessages[feed.id] ?? ''}
    {@const feedErrorID = `feed-url-${feed.id}-error`}
    <fieldset class="grid min-w-0 gap-3 rounded-md border border-line p-4 sm:grid-cols-2">
      <legend class="px-1 text-sm font-semibold">Feed</legend>
      <label class="grid min-w-0 gap-2 text-sm"
        >名称<input
          class="min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3"
          bind:value={feed.name}
          placeholder="默认订阅"
        /></label
      >
      <label class="grid min-w-0 gap-2 text-sm"
        >格式<select
          class="min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3"
          bind:value={feed.format}
          >{#each feedFormats as format (format)}<option value={format}>{format}</option
            >{/each}</select
        ></label
      >
      <label class="grid min-w-0 gap-2 text-sm sm:col-span-2"
        >地址<input
          class={[
            'min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3',
            feedMessage && 'border-danger',
          ]}
          bind:value={feed.url}
          placeholder="https://example.com/feed.xml"
          aria-invalid={feedMessage ? 'true' : undefined}
          aria-describedby={feedMessage ? feedErrorID : undefined}
        />{#if feedMessage}<p id={feedErrorID} class="text-xs text-danger-fg">
            {feedMessage}
          </p>{/if}</label
      >
      <label class="flex min-h-11 items-center gap-3 text-sm"
        ><input
          class="size-4 accent-primary"
          type="radio"
          name="default-feed"
          checked={feed.isDefault}
          onchange={() => setDefaultFeed(form, feed.id)}
        />默认 Feed</label
      >
      <button
        class="inline-flex min-h-11 items-center justify-center gap-2 rounded-sm border border-danger px-3 text-sm font-medium text-danger-fg"
        type="button"
        onclick={() => removeFeed(form, feed.id)}
        ><IconTrash size={18} aria-hidden="true" />删除</button
      >
    </fieldset>
  {/each}
  <div class="grid gap-4 sm:grid-cols-2">
    <label class="grid min-w-0 gap-2 text-sm"
      >Sitemap（选填）<input
        class={[
          'min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3',
          urlValidation.sitemapMessage && 'border-danger',
        ]}
        bind:value={form.sitemap}
        placeholder="https://example.com/sitemap.xml"
        aria-invalid={urlValidation.sitemapMessage ? 'true' : undefined}
        aria-describedby={`sitemap-url-help${urlValidation.sitemapMessage ? ' sitemap-url-error' : ''}`}
      />
      <p id="sitemap-url-help" class="text-xs text-fg-muted">填写 XML Sitemap 地址；没有时留空。</p>
      {#if urlValidation.sitemapMessage}<p id="sitemap-url-error" class="text-xs text-danger-fg">
          {urlValidation.sitemapMessage}
        </p>{/if}</label
    >
    <label class="grid min-w-0 gap-2 text-sm"
      >友链页（选填）<input
        class={[
          'min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3',
          urlValidation.linkPageMessage && 'border-danger',
        ]}
        bind:value={form.linkPage}
        placeholder="https://example.com/friends"
        aria-invalid={urlValidation.linkPageMessage ? 'true' : undefined}
        aria-describedby={`link-page-url-help${urlValidation.linkPageMessage ? ' link-page-url-error' : ''}`}
      />
      <p id="link-page-url-help" class="text-xs text-fg-muted">
        填写展示友情链接的页面地址；没有时留空。
      </p>
      {#if urlValidation.linkPageMessage}<p id="link-page-url-error" class="text-xs text-danger-fg">
          {urlValidation.linkPageMessage}
        </p>{/if}</label
    >
  </div>
</section>

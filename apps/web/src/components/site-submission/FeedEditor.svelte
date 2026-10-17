<script lang="ts">
  import { IconPlus, IconTrash } from '@tabler/icons-svelte';

  import { feedFormats } from '@/api/site-submission/site-submission.types';
  import {
    addFeed,
    type EditableSubmission,
    removeFeed,
    setDefaultFeed,
  } from '@/application/site-submission/site-submission.browser';
  import { validateAuxiliaryURLs } from '@/application/site-submission/site-submission.validation';
  interface Props {
    form: EditableSubmission;
  }
  let { form = $bindable() }: Props = $props();
  let urlValidation = $derived(validateAuxiliaryURLs(form));
</script>

<section class="grid min-w-0 gap-4" data-field-group="feeds">
  <header class="flex min-w-0 flex-wrap items-start justify-between gap-4">
    <div class="min-w-0">
      <h2 class="text-xl font-semibold">Feed 与站点资源</h2>
      <p class="mt-1 text-sm text-fg-muted">
        <span class="block">最多添加 8 条 Feed，并选择一个默认 Feed；没有时可以留空。</span>
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
  <p id="feed-address-help" class="text-xs text-fg-muted">
    Feed 地址支持 HTTP、HTTPS 或站内相对地址，不能与主页相同或重复。
  </p>
  {#each form.feeds as feed (feed.id)}
    <fieldset class="grid min-w-0 gap-3 rounded-md border border-line p-4 sm:grid-cols-2">
      <legend class="px-1 text-sm font-semibold">Feed</legend>
      <label class="grid min-w-0 gap-2 text-sm"
        >名称<input
          class="min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3"
          bind:value={feed.name}
          name={`feed-name-${feed.id}`}
          data-max-bytes="100"
          required={Boolean(feed.url.trim())}
          aria-describedby={`feed-name-${feed.id}-help`}
          placeholder="默认订阅"
        /><span id={`feed-name-${feed.id}-help`} class="text-xs text-fg-muted"
          >填写地址后名称必填，最多 100 字节。</span
        ></label
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
          class="min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3"
          bind:value={feed.url}
          name={`feed-url-${feed.id}`}
          placeholder="https://example.com/feed.xml"
          data-custom-issue={urlValidation.feedMessages[feed.id] ?? ''}
          aria-describedby="feed-address-help"
        /></label
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
        class="min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3"
        bind:value={form.sitemap}
        name="sitemap"
        placeholder="https://example.com/sitemap.xml"
        data-custom-issue={urlValidation.sitemapMessage}
        aria-describedby="sitemap-url-help"
      />
      <p id="sitemap-url-help" class="text-xs text-fg-muted">填写 XML Sitemap 地址；没有时留空。</p>
    </label>
    <label class="grid min-w-0 gap-2 text-sm"
      >友链页（选填）<input
        class="min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3"
        bind:value={form.linkPage}
        name="linkPage"
        placeholder="https://example.com/friends"
        data-custom-issue={urlValidation.linkPageMessage}
        aria-describedby="link-page-url-help"
      />
      <p id="link-page-url-help" class="text-xs text-fg-muted">
        填写展示友情链接的页面地址；没有时留空。
      </p>
    </label>
  </div>
</section>

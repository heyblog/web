<script lang="ts">
  import { IconPlus, IconTrash } from '@tabler/icons-svelte';

  import {
    addFeed,
    type EditableSubmission,
    removeFeed,
    setDefaultFeed,
  } from '@/application/site-submission/site-submission.browser';
  import { feedFormats } from '@/application/site-submission/site-submission.types';
  interface Props {
    form: EditableSubmission;
  }
  let { form }: Props = $props();
</script>

<section class="grid gap-4">
  <header class="flex items-start justify-between gap-4">
    <div>
      <h2 class="text-xl font-semibold">Feed 与站点资源</h2>
      <p class="mt-1 text-sm text-fg-muted">最多添加 8 个 Feed；添加后必须指定一个默认项。</p>
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
      没有 Feed 也可以继续。
    </p>{/if}
  {#each form.feeds as feed (feed.id)}
    <fieldset class="grid gap-3 rounded-md border border-line p-4 sm:grid-cols-2">
      <legend class="px-1 text-sm font-semibold">Feed</legend>
      <label class="grid gap-2 text-sm"
        >名称<input
          class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
          bind:value={feed.name}
          placeholder="默认订阅"
        /></label
      >
      <label class="grid gap-2 text-sm"
        >格式<select
          class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
          bind:value={feed.format}
          >{#each feedFormats as format (format)}<option value={format}>{format}</option
            >{/each}</select
        ></label
      >
      <label class="grid gap-2 text-sm sm:col-span-2"
        >地址<input
          class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
          bind:value={feed.url}
          placeholder="https://example.com/feed.xml"
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
    <label class="grid gap-2 text-sm"
      >Sitemap<input
        class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
        bind:value={form.sitemap}
        placeholder="https://example.com/sitemap.xml"
      /></label
    >
    <label class="grid gap-2 text-sm"
      >友链页<input
        class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
        bind:value={form.linkPage}
        placeholder="https://example.com/friends"
      /></label
    >
  </div>
</section>

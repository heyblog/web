<script lang="ts">
  import { tick } from 'svelte';

  import { type IncomingLink } from '@/api/site-annotation/annotation.types';
  import type { AnnotationDraft } from '@/application/site-annotation/annotation-draft';
  import {
    annotationDetailPath,
    linkStatusLabels,
  } from '@/application/site-annotation/annotation-view';

  import { buttonClass, inputClass, linkClass, sectionClass } from './annotation.styles';

  let {
    draft = $bindable(),
    incoming,
    allocateKey,
    section = 'all',
    siteUrl = '',
    onPreview,
  }: {
    draft: AnnotationDraft;
    incoming: readonly IncomingLink[];
    allocateKey: () => number;
    section?: 'all' | 'outgoing' | 'incoming';
    siteUrl?: string;
    onPreview?: (url: string) => void;
  } = $props();

  let filterText = $state('');
  let expanded = $state(false);
  let listContainer = $state<HTMLDivElement | null>(null);

  const normalizedSiteHost = $derived(extractHost(siteUrl));
  const hostCounts = $derived.by(() => {
    const counts: Record<string, number> = {};
    for (const item of draft.outgoing) {
      const host = extractHost(item.target_url);
      if (host) counts[host] = (counts[host] ?? 0) + 1;
    }
    return counts;
  });

  const filteredOutgoing = $derived.by(() => {
    const query = filterText.trim().toLowerCase();
    if (!query) return draft.outgoing;
    return draft.outgoing.filter((item) => item.target_url.toLowerCase().includes(query));
  });

  function extractHost(value: string): string {
    const trimmed = value.trim();
    if (!trimmed) return '';
    try {
      const candidate = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
      return new URL(candidate).hostname.toLowerCase().replace(/\.$/u, '');
    } catch {
      return '';
    }
  }

  function externalHref(value: string): string | null {
    const trimmed = value.trim();
    return /^https?:\/\//i.test(trimmed) ? trimmed : null;
  }

  function normalizeLinkUrl(link: AnnotationDraft['outgoing'][number]) {
    const trimmed = link.target_url.trim();
    if (!trimmed) return;
    if (!/^[a-zA-Z][a-zA-Z\d+.-]*:\/\//u.test(trimmed) && /^[^\s/]+\.[^\s]+$/u.test(trimmed)) {
      link.target_url = `https://${trimmed}`;
    } else if (trimmed !== link.target_url) {
      link.target_url = trimmed;
    }
  }

  function linkValidationError(link: AnnotationDraft['outgoing'][number]): string {
    const trimmed = link.target_url.trim();
    if (!trimmed) return '';
    const host = extractHost(trimmed);
    if (!host) return '网址格式无效，请检查后重试。';
    if (normalizedSiteHost && host === normalizedSiteHost) return '不能将当前站点自身添加为友链。';
    if ((hostCounts[host] ?? 0) > 1) return `域名 ${host} 已在友链列表中重复。`;
    return '';
  }

  async function focusLinkInput(key: number) {
    await tick();
    const input = listContainer?.querySelector<HTMLInputElement>(`input[data-link-key="${key}"]`);
    input?.focus();
  }

  function addLink(insertAt = 0) {
    filterText = '';
    const key = allocateKey();
    draft.outgoing.splice(insertAt, 0, { key, target_url: '', link_status: 'ACTIVE' });
    void focusLinkInput(key);
  }

  function handleLinkKeydown(event: KeyboardEvent, link: AnnotationDraft['outgoing'][number]) {
    if (event.key !== 'Enter' || event.isComposing) return;
    event.preventDefault();
    normalizeLinkUrl(link);
    if (!link.target_url.trim()) return;
    const currentIndex = draft.outgoing.findIndex((entry) => entry.key === link.key);
    addLink(currentIndex >= 0 ? currentIndex + 1 : 0);
  }

  function toggleStatus(link: AnnotationDraft['outgoing'][number]) {
    link.link_status = link.link_status === 'ACTIVE' ? 'INACTIVE' : 'ACTIVE';
  }
</script>

{#if section === 'all' || section === 'outgoing'}
  <section class={sectionClass} aria-labelledby="annotation-outgoing">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-baseline gap-2">
        <h2 id="annotation-outgoing" class="text-lg font-semibold">友链</h2>
        <span class="text-sm text-fg-muted">共 {draft.outgoing.length} 条</span>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        {#if draft.outgoing.length > 8}
          <button
            class={buttonClass}
            type="button"
            onclick={() => {
              expanded = !expanded;
            }}
          >
            {expanded ? '收起列表' : '展开全部'}
          </button>
        {/if}
        <button class={buttonClass} type="button" onclick={() => addLink(0)}>添加友链</button>
      </div>
    </div>

    {#if draft.outgoing.length > 5}
      <input
        class={inputClass}
        type="search"
        aria-label="筛选友链"
        placeholder="搜索已录入的友链网址…"
        bind:value={filterText}
      />
    {/if}

    {#if draft.outgoing.length === 0}
      <p class="text-sm text-fg-muted">暂无友链。</p>
    {:else if filteredOutgoing.length === 0}
      <p class="text-sm text-fg-muted">没有匹配的友链。</p>
    {:else}
      <div
        bind:this={listContainer}
        class:max-h-96={!expanded && filteredOutgoing.length > 8}
        class:overflow-y-auto={!expanded && filteredOutgoing.length > 8}
        class="grid min-w-0 gap-2 pr-1"
      >
        {#each filteredOutgoing as link (link.key)}
          {@const href = externalHref(link.target_url)}
          {@const validationError = linkValidationError(link)}
          {@const active = link.link_status === 'ACTIVE'}
          <div class="grid min-w-0 gap-1">
            <div class="flex flex-wrap items-center gap-2 sm:flex-nowrap">
              <input
                class={`${inputClass} flex-1`}
                class:border-danger={Boolean(validationError)}
                type="url"
                data-link-key={link.key}
                aria-label="友链目标网址"
                placeholder="https://example.com（回车添加下一条）"
                bind:value={link.target_url}
                onkeydown={(event) => handleLinkKeydown(event, link)}
                onblur={() => normalizeLinkUrl(link)}
                onpaste={() => {
                  queueMicrotask(() => normalizeLinkUrl(link));
                }}
                maxlength="2048"
                required
              />
              <div class="flex shrink-0 items-center gap-1.5">
                <button
                  class:border-success={active}
                  class:bg-success-bg={active}
                  class:text-success-fg={active}
                  class:border-line-strong={!active}
                  class:bg-subtle={!active}
                  class:text-fg-muted={!active}
                  class="inline-flex min-h-11 cursor-pointer items-center justify-center rounded-md border px-3 text-xs font-medium hover:opacity-85 sm:min-h-10"
                  type="button"
                  aria-pressed={active}
                  onclick={() => toggleStatus(link)}
                >
                  {linkStatusLabels[link.link_status]}
                </button>
                {#if href && onPreview}
                  <button
                    class="inline-flex min-h-11 cursor-pointer items-center justify-center rounded-md border border-line-strong bg-surface px-2.5 text-xs font-medium text-tint-fg hover:bg-subtle sm:min-h-10"
                    type="button"
                    onclick={() => onPreview(href)}>右窗</button
                  >
                {/if}
                {#if href}
                  <a
                    class="inline-flex min-h-11 items-center justify-center rounded-md border border-line-strong bg-surface px-2.5 text-xs font-medium text-tint-fg hover:bg-subtle sm:min-h-10"
                    {href}
                    target="_blank"
                    rel="noopener noreferrer"
                    aria-label="新标签页打开友链">↗</a
                  >
                {/if}
                {#if link.key < 0}
                  <button
                    class="inline-flex min-h-11 cursor-pointer items-center justify-center rounded-md border border-line-strong bg-surface px-2.5 text-xs font-medium text-fg-muted hover:bg-subtle hover:text-danger-fg sm:min-h-10"
                    type="button"
                    aria-label="移除友链"
                    onclick={() => {
                      draft.outgoing = draft.outgoing.filter((entry) => entry.key !== link.key);
                    }}>×</button
                  >
                {/if}
              </div>
            </div>
            {#if validationError}
              <p class="text-xs text-danger-fg" role="alert">{validationError}</p>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </section>
{/if}

{#if section === 'all' || section === 'incoming'}
  <section class={sectionClass} aria-labelledby="annotation-incoming">
    <h2 id="annotation-incoming" class="text-lg font-semibold">指向本站的友链</h2>
    {#if incoming.length === 0}
      <p class="text-sm text-fg-muted">暂无站点链接到这里。</p>
    {:else}
      <ul class="divide-y divide-line">
        {#each incoming as link (`${link.source_short_id}:${link.source_url}`)}
          <li class="grid min-w-0 gap-1 py-2 text-sm">
            {#if link.source_short_id}
              <a
                class={`${linkClass} wrap-anywhere`}
                href={annotationDetailPath(link.source_short_id)}
                >{link.source_name || link.source_short_id}</a
              >
            {:else}
              <span class="font-medium wrap-anywhere">{link.source_name || link.source_url}</span>
            {/if}
            <span class="wrap-anywhere text-fg-muted"
              >{link.source_url} · {linkStatusLabels[link.link_status]}</span
            >
          </li>
        {/each}
      </ul>
    {/if}
  </section>
{/if}

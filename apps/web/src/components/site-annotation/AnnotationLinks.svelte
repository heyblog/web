<script lang="ts">
  import { type IncomingLink, linkStatuses } from '@/api/site-annotation/annotation.types';
  import type { AnnotationDraft } from '@/application/site-annotation/annotation-draft';
  import {
    annotationDetailPath,
    linkStatusLabels,
  } from '@/application/site-annotation/annotation-view';

  import {
    buttonClass,
    fieldClass,
    inputClass,
    linkClass,
    rowClass,
    sectionClass,
  } from './annotation.styles';

  let {
    draft = $bindable(),
    incoming,
    allocateKey,
    section = 'all',
  }: {
    draft: AnnotationDraft;
    incoming: readonly IncomingLink[];
    allocateKey: () => number;
    section?: 'all' | 'outgoing' | 'incoming';
  } = $props();

  function addLink() {
    draft.outgoing.push({ key: allocateKey(), target_url: '', link_status: 'ACTIVE' });
  }

  function externalHref(value: string): string | null {
    const trimmed = value.trim();
    return /^https?:\/\//i.test(trimmed) ? trimmed : null;
  }
</script>

{#if section === 'all' || section === 'outgoing'}
  <section class={sectionClass} aria-labelledby="annotation-outgoing">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-baseline gap-2">
        <h2 id="annotation-outgoing" class="text-lg font-semibold">友链</h2>
        <span class="text-sm text-fg-muted">共 {draft.outgoing.length} 条</span>
      </div>
      <button class={buttonClass} type="button" onclick={addLink}>添加友链</button>
    </div>
    {#if draft.outgoing.length === 0}<p class="text-sm text-fg-muted">暂无友链。</p>{/if}
    {#each draft.outgoing as link (link.key)}
      {@const href = externalHref(link.target_url)}
      <div class={rowClass}>
        <div class={fieldClass}>
          <div class="flex items-center justify-between gap-2">
            <span>目标网址</span>
            {#if href}
              <a
                class="text-xs font-medium text-tint-fg hover:underline"
                {href}
                target="_blank"
                rel="noopener noreferrer">新标签页打开 ↗</a
              >
            {/if}
          </div>
          <input
            class={inputClass}
            type="url"
            aria-label="目标网址"
            bind:value={link.target_url}
            maxlength="2048"
            required
          />
        </div>
        <div class="flex items-end gap-3">
          <label class={`${fieldClass} flex-1`}
            >状态
            <select class={inputClass} bind:value={link.link_status}>
              {#each linkStatuses as status (status)}<option value={status}
                  >{linkStatusLabels[status]}</option
                >{/each}
            </select>
          </label>
          {#if link.key < 0}
            <button
              class={buttonClass}
              type="button"
              onclick={() => {
                draft.outgoing = draft.outgoing.filter((entry) => entry.key !== link.key);
              }}>移除</button
            >
          {/if}
        </div>
      </div>
    {/each}
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

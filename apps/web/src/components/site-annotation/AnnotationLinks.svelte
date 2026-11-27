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
  }: {
    draft: AnnotationDraft;
    incoming: readonly IncomingLink[];
    allocateKey: () => number;
  } = $props();

  function addLink() {
    draft.outgoing.push({ key: allocateKey(), target_url: '', link_status: 'ACTIVE' });
  }
</script>

<section class={sectionClass} aria-labelledby="annotation-outgoing">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <h2 id="annotation-outgoing" class="text-lg font-semibold">本站的友链</h2>
    <button class={buttonClass} type="button" onclick={addLink}>添加友链</button>
  </div>
  {#if draft.outgoing.length === 0}<p class="text-sm text-fg-muted">暂无友链。</p>{/if}
  {#each draft.outgoing as link (link.key)}
    <div class={rowClass}>
      <label class={fieldClass}
        >目标网址<input
          class={inputClass}
          type="url"
          bind:value={link.target_url}
          maxlength="2048"
          required
        /></label
      >
      <label class={fieldClass}
        >状态
        <select class={inputClass} bind:value={link.link_status}>
          {#each linkStatuses as status (status)}<option value={status}
              >{linkStatusLabels[status]}</option
            >{/each}
        </select>
      </label>
      {#if link.key < 0}
        <button
          class={`${buttonClass} justify-self-start`}
          type="button"
          onclick={() => {
            draft.outgoing = draft.outgoing.filter((entry) => entry.key !== link.key);
          }}>移除友链</button
        >
      {/if}
    </div>
  {/each}
</section>

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

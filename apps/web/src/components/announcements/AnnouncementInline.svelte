<script lang="ts">
  import {
    type InlineNode,
    parseAnnouncementInline,
  } from '../../application/announcements/announcement-inline.shared.ts';

  let { source, interactiveLinks = true }: { source: string; interactiveLinks?: boolean } =
    $props();
  const nodes = $derived(parseAnnouncementInline(source));
</script>

{#snippet content(items: readonly InlineNode[])}
  {#each items as item (item)}
    {#if item.kind === 'text'}{item.text}
    {:else if item.kind === 'code'}<code class="rounded-sm bg-subtle px-1 font-mono text-sm"
        >{item.text}</code
      >
    {:else if item.kind === 'strong'}<strong>{@render content(item.children)}</strong>
    {:else if item.kind === 'em'}<em>{@render content(item.children)}</em>
    {:else if item.kind === 'del'}<del>{@render content(item.children)}</del>
    {:else if item.kind === 'link' && interactiveLinks}<a
        class="text-tint-fg underline underline-offset-4"
        href={item.href}
        target={item.external ? '_blank' : undefined}
        rel={item.external ? 'noopener noreferrer' : undefined}>{@render content(item.children)}</a
      >
    {:else if item.kind === 'link'}{@render content(item.children)}{/if}
  {/each}
{/snippet}

<span class="wrap-anywhere whitespace-pre-wrap">{@render content(nodes)}</span>

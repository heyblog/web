<script lang="ts">
  import { IconArrowRight, IconMessageCircle } from '@tabler/icons-svelte';

  import {
    formatSiteJoinedAt,
    type HomeSiteCard,
    siteDetailPath,
  } from '@/application/home/home.shared';

  import BlogCardHint from './BlogCardHint.svelte';
  import BlogCardResourceLinks from './BlogCardResourceLinks.svelte';

  type Props = {
    readonly site: HomeSiteCard;
    readonly expanded: boolean;
    readonly feedback?: boolean;
  };

  const detailActionClass =
    'pointer-events-auto relative z-10 inline-flex min-h-11 items-center gap-1 rounded-md px-2 text-sm font-medium whitespace-nowrap text-tint-fg outline-none transition-colors duration-(--motion-fast) hover:bg-tint focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-offset-2 focus-visible:ring-offset-surface sm:min-h-10';

  let { site, expanded, feedback }: Props = $props();
</script>

<footer
  class={[
    'pointer-events-none relative z-10 mt-2 flex min-h-10 items-center gap-1 pt-1',
    !expanded && 'border-t border-line',
  ]}
>
  <time class="mr-auto truncate text-xs text-fg-muted" datetime={site.joinedAt}
    >{formatSiteJoinedAt(site.joinedAt)}</time
  >
  <BlogCardResourceLinks {site} />
  {#if feedback}
    <BlogCardHint
      label="反馈（暂未开放）"
      accessibleLabel={`${site.name} 反馈（暂未开放）`}
      className="size-11 shrink-0 items-center justify-center text-fg-muted sm:size-10"
    >
      <IconMessageCircle aria-hidden="true" size={16} stroke={1.8} />
    </BlogCardHint>
  {/if}
  <a class={detailActionClass} href={siteDetailPath(site)}>
    详情
    <IconArrowRight aria-hidden="true" size={15} stroke={1.8} />
  </a>
</footer>

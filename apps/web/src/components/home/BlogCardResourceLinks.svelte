<script lang="ts">
  import { IconRss, IconSitemap } from '@tabler/icons-svelte';

  import type { HomeSiteCard } from '@/application/home/home.shared';

  import BlogCardHint from './BlogCardHint.svelte';

  interface Props {
    site: HomeSiteCard;
  }

  let { site }: Props = $props();
  const feedLabel = $derived(
    site.defaultFeed?.format === 'UNKNOWN' ? '订阅' : `${site.defaultFeed?.format} 订阅`,
  );
</script>

{#if site.defaultFeed}
  <BlogCardHint
    label={feedLabel}
    accessibleLabel={`${site.name} ${feedLabel}`}
    className="size-11 shrink-0 items-center justify-center text-fg-muted sm:size-10"
  >
    <IconRss aria-hidden="true" size={16} stroke={1.8} />
  </BlogCardHint>
{/if}
{#if site.sitemapUrl}
  <BlogCardHint
    label="站点地图"
    accessibleLabel={`${site.name} 站点地图`}
    className="size-11 shrink-0 items-center justify-center text-fg-muted sm:size-10"
  >
    <IconSitemap aria-hidden="true" size={16} stroke={1.8} />
  </BlogCardHint>
{/if}

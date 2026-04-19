<script lang="ts">
  import {
    IconArrowRight,
    IconExternalLink,
    IconMessageCircle,
    IconRss,
    IconSitemap,
    IconStarFilled,
  } from '@tabler/icons-svelte-runes';

  import { trackSiteAccess } from '@/application/site/site-access.client';
  import type { BlogCardUpdatedTone, SiteCardEntry } from '@/application/site/site-card.shared';
  import SiteTagRow from '@/components/site/SiteTagRow.svelte';

  let {
    entry,
    onFeedback,
  }: {
    entry: SiteCardEntry;
    onFeedback?: (entry: SiteCardEntry) => void;
  } = $props();

  type ResourceLink = {
    href: string;
    label: string;
    title: string;
    kind: 'rss' | 'sitemap';
    targetKind: 'FEED' | 'SITEMAP';
  };

  const detailHref = $derived(`/site/${entry.slug}`);
  const toneClass = {
    amber: '[--card-accent:var(--color-warn)] [--card-accent-dot:var(--color-warn-dot)]',
    blue: '[--card-accent:var(--color-info)] [--card-accent-dot:var(--color-info-dot)]',
    emerald: '[--card-accent:var(--color-ok)] [--card-accent-dot:var(--color-ok-dot)]',
    red: '[--card-accent:var(--color-fail)] [--card-accent-dot:var(--color-fail-dot)]',
    stone: '[--card-accent:var(--color-fg-2)] [--card-accent-dot:var(--color-fg-3)]',
  } satisfies Record<SiteCardEntry['tone'], string>;

  const updatedToneClass = {
    amber: '[--card-updated-fg:var(--color-warn)] [--card-updated-dot:var(--color-warn-dot)]',
    blue: '[--card-updated-fg:var(--color-info)] [--card-updated-dot:var(--color-info-dot)]',
    emerald: '[--card-updated-fg:var(--color-ok)] [--card-updated-dot:var(--color-ok-dot)]',
    stone: '[--card-updated-fg:var(--color-fg-3)] [--card-updated-dot:var(--color-fg-3)]',
  } satisfies Record<BlogCardUpdatedTone, string>;

  const resourceLinks = $derived.by<ResourceLink[]>(() => {
    const links: ResourceLink[] = [];

    if (entry.rssUrl) {
      links.push({
        href: entry.rssUrl,
        label: `${entry.name} RSS 订阅`,
        title: 'RSS 订阅',
        kind: 'rss',
        targetKind: 'FEED',
      });
    }

    if (entry.sitemapUrl) {
      links.push({
        href: entry.sitemapUrl,
        label: `${entry.name} 站点地图`,
        title: '站点地图',
        kind: 'sitemap',
        targetKind: 'SITEMAP',
      });
    }

    return links;
  });

  const metrics = $derived.by(() => {
    const items = [{ label: '访问', value: entry.visitCount }];

    if (entry.articleCount) {
      items.push({ label: '文章', value: entry.articleCount });
    }

    return items;
  });

  const iconActionClass =
    'inline-flex size-6.5 items-center justify-center rounded-[4px] text-[color:var(--color-fg-3)] transition-[background-color,color] duration-150 ease-out-smooth hover:bg-[color:var(--color-bg-raised)] hover:text-[color:var(--card-accent)] focus-visible:outline-none focus-visible:bg-[color:var(--color-bg-raised)]';
  const metricLabelClass =
    'font-mono text-[10px] leading-[1.2] tracking-[0.16em] text-[color:var(--color-fg-3)] uppercase';
  const metricValueClass =
    'mt-1 text-[14px] leading-[1.2] font-medium text-[color:var(--color-fg)]';
</script>

<article
  class={[
    'flex h-64 flex-col overflow-hidden rounded-md border border-(--color-line-med) bg-transparent p-2 transition-[background-color,border-color,transform] duration-150 ease-out-smooth hover:-translate-y-px hover:bg-(--color-bg-raised) hover:border-[color-mix(in_srgb,var(--card-accent)_18%,var(--color-line-med))] xs:h-63 xs:p-5',
    '[--card-accent:var(--color-info)] [--card-accent-dot:var(--color-info-dot)]',
    '[--card-updated-fg:var(--color-ok)] [--card-updated-dot:var(--color-ok-dot)]',
    toneClass[entry.tone],
    entry.updatedTone ? updatedToneClass[entry.updatedTone] : '',
  ].join(' ')}
>
  <div class="flex min-h-0 flex-1 flex-col">
    <header class="flex items-start gap-1.5">
      <span
        class="inline-flex size-8 shrink-0 items-center justify-center rounded-sm bg-[color-mix(in_srgb,var(--card-accent)_8%,var(--color-bg-raised))] font-mono text-[11px] leading-none tracking-[0.14em] text-(--card-accent) uppercase"
        aria-hidden="true"
      >
        {entry.shortCode}
      </span>

      <div class="min-w-0 flex-1">
        <a
          class="group block max-w-full"
          href={entry.href}
          rel="noreferrer"
          target="_blank"
          onclick={() => {
            trackSiteAccess(entry.id, {
              source: 'SITE_CARD',
              targetKind: 'SITE',
            });
          }}
        >
          <div class="flex flex-wrap items-start gap-x-1 gap-y-0.5">
            <h2
              class="overflow-hidden text-[15px] leading-[1.3] font-medium text-(--color-fg) transition-colors duration-150 ease-out-smooth [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2] group-hover:text-(--card-accent)"
            >
              {entry.name}
            </h2>
            <span class="mt-0.5 inline-flex items-center text-(--card-accent)" aria-hidden="true">
              <IconExternalLink size={13} stroke={1.9} class="block" />
            </span>
            {#if entry.featured}
              <span
                class="mt-0.5 inline-flex items-center text-(--card-accent)"
                aria-label="推荐"
                title="推荐"
              >
                <IconStarFilled size={13} class="block" />
              </span>
            {/if}
          </div>
          <p
            class="mt-0.75 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-[11px] leading-[1.35] text-(--color-fg-3) transition-colors duration-150 ease-out-smooth group-hover:text-(--card-accent)"
          >
            {entry.domain}
          </p>
        </a>
      </div>
    </header>

    <div class="mt-2 flex min-h-0 flex-1 flex-col">
      <div class="flex min-h-0 flex-1 flex-col">
        <div class="flex h-[3.36rem] items-center overflow-hidden">
          <p
            class="overflow-hidden text-base leading-[1.68] text-(--color-fg-2) [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
          >
            {entry.summary}
          </p>
        </div>

        <div class="my-auto overflow-hidden">
          <SiteTagRow
            primaryTag={entry.primaryTag}
            subTags={entry.subTags}
            warningTags={entry.warningTags}
            tone={entry.tone}
            compact={true}
            singleLine={true}
          />
        </div>
      </div>

      <div class="mt-2 h-19 shrink-0 border-t border-(--color-line) pt-2">
        <div class="flex h-full flex-col">
          <div class="flex min-h-6.5 items-center justify-between gap-2">
            <div class="flex min-w-0 items-center gap-2 overflow-hidden">
              <p
                class="inline-flex shrink-0 items-center gap-1.5 font-mono text-[10px] leading-[1.35] text-(--color-fg-3)"
              >
                <span>加入</span>
                <time datetime={entry.joinedAt}>{entry.joinedLabel}</time>
              </p>

              {#if entry.updatedLabel}
                <p
                  class="inline-flex min-w-0 items-center gap-1.5 overflow-hidden font-mono text-[10px] leading-[1.35] text-(--card-updated-fg)"
                >
                  <span
                    class="size-1.5 shrink-0 rounded-full bg-(--card-updated-dot)"
                    aria-hidden="true"
                  ></span>
                  <span class="truncate">{entry.updatedLabel}</span>
                </p>
              {/if}
            </div>

            {#if resourceLinks.length > 0}
              <div
                class="inline-flex shrink-0 items-center gap-1"
                aria-label={`${entry.name} 资源链接`}
              >
                {#each resourceLinks as link (link.href)}
                  <a
                    class={iconActionClass}
                    href={link.href}
                    rel="noreferrer"
                    target="_blank"
                    aria-label={link.label}
                    title={link.title}
                    onclick={() => {
                      trackSiteAccess(entry.id, {
                        source: 'SITE_CARD',
                        targetKind: link.targetKind,
                      });
                    }}
                  >
                    {#if link.kind === 'rss'}
                      <IconRss size={14} stroke={1.8} class="block" />
                    {:else}
                      <IconSitemap size={14} stroke={1.8} class="block" />
                    {/if}
                  </a>
                {/each}
              </div>
            {/if}
          </div>

          <footer class="mt-auto flex min-h-9 items-end justify-between gap-2">
            <dl class="flex min-w-0 items-end gap-4 overflow-hidden">
              {#each metrics as metric (metric.label)}
                <div class="min-w-18">
                  <dt class={metricLabelClass}>{metric.label}</dt>
                  <dd class={metricValueClass}>{metric.value}</dd>
                </div>
              {/each}
            </dl>

            <div class="inline-flex shrink-0 items-center gap-1.5">
              {#if onFeedback}
                <button
                  class="inline-flex min-h-6.5 shrink-0 items-center gap-1 rounded-sm px-2.5 py-1 text-[12px] leading-none font-medium text-(--color-fg-2) transition-[background-color,color] duration-150 ease-out-smooth hover:bg-(--color-bg-raised) hover:text-(--card-accent) focus-visible:outline-none focus-visible:bg-(--color-bg-raised)"
                  type="button"
                  onclick={() => onFeedback?.(entry)}
                  aria-label={`反馈 ${entry.name} 的站点问题`}
                  title="反馈站点问题"
                >
                  反馈
                  <IconMessageCircle size={13} stroke={1.9} class="block" />
                </button>
              {/if}
              <a
                class="inline-flex min-h-6.5 shrink-0 items-center gap-1 rounded-sm px-2.5 py-1 text-[12px] leading-none font-medium text-(--color-fg-2) transition-[background-color,color] duration-150 ease-out-smooth hover:bg-(--color-bg-raised) hover:text-(--card-accent) focus-visible:outline-none focus-visible:bg-(--color-bg-raised)"
                href={detailHref}
              >
                详情
                <IconArrowRight size={13} stroke={1.9} class="block" />
              </a>
            </div>
          </footer>
        </div>
      </div>
    </div>
  </div>
</article>

<script lang="ts">
  import { IconExternalLink, IconMessageCircle } from '@tabler/icons-svelte-runes';

  import { trackSiteAccess } from '@/application/site/site-access.client';
  import type { BlogCardTone } from '@/application/site/site-card.shared';
  import type { SiteCheckItem, SiteDetail } from '@/application/site/site-directory.models';
  import {
    buildSiteCheckFacts,
    formatSiteCheckRegionLabel,
    formatSiteCheckResultLabel,
    formatSiteDetailStatusLabel,
    resolveSiteCheckTextClass,
    resolveSiteCheckTone,
    resolveSiteDetailDescription,
  } from '@/components/site/site-detail.shared';
  import SiteTagRow from '@/components/site/SiteTagRow.svelte';

  type HeartbeatEntry = {
    id: string;
    item: SiteCheckItem | null;
  };

  let {
    detail,
    tagTone,
    statusToneClass,
    heartbeatChecks,
    onOpenFeedback,
  }: {
    detail: SiteDetail;
    tagTone: BlogCardTone;
    statusToneClass: string;
    heartbeatChecks: HeartbeatEntry[];
    onOpenFeedback?: () => void;
  } = $props();
</script>

<div class="min-w-0 pt-2">
  <div class="flex flex-wrap items-start justify-between gap-4">
    <h1
      class="min-w-0 text-[clamp(2.3rem,6vw,4.4rem)] leading-[0.94] tracking-[-0.05em] text-(--color-fg)"
    >
      {detail.name}
    </h1>
    <button
      class="inline-flex h-10 shrink-0 items-center gap-2 rounded-md px-0 text-sm text-(--color-fg-2) transition hover:text-(--color-fg)"
      type="button"
      onclick={onOpenFeedback}
    >
      <IconMessageCircle size={15} stroke={1.8} />
      <span>反馈站点问题</span>
    </button>
  </div>
  <a
    class="mt-4 inline-flex items-center gap-2 text-sm text-(--color-fg-3) transition hover:text-(--color-fg)"
    href={detail.url}
    rel="noreferrer"
    target="_blank"
    onclick={() => {
      trackSiteAccess(detail.id, {
        source: 'SITE_DETAIL',
        targetKind: 'SITE',
      });
    }}
  >
    <span>{detail.url}</span>
    <IconExternalLink size={15} stroke={1.8} />
  </a>
  <p class="mt-6 max-w-3xl text-[16px] leading-8 text-(--color-fg-2)">
    {resolveSiteDetailDescription(detail.sign)}
  </p>
  <div class="mt-4">
    <SiteTagRow
      primaryTag={detail.primaryTag}
      subTags={detail.subTags}
      warningTags={detail.warningTags}
      tone={tagTone}
    />
  </div>

  <div class="mt-8 border-t border-(--color-line) pt-5">
    <div class="flex flex-wrap items-center justify-between gap-x-6 gap-y-4">
      <div class="min-w-32 space-y-2">
        <p class="font-mono text-[10px] tracking-[0.16em] text-(--color-fg-3) uppercase">状态</p>
        <p class={`text-base font-medium ${statusToneClass}`}>
          {formatSiteDetailStatusLabel(detail.status)}
        </p>
      </div>

      <div class="ml-auto min-w-0 flex-1">
        <div class="flex min-h-8 justify-end gap-1.5">
          {#each heartbeatChecks as entry (entry.id)}
            <div class="group relative">
              <span
                class="block h-8 w-2 rounded-xs"
                style={`background:${entry.item ? resolveSiteCheckTone(entry.item.result) : 'var(--color-fg-3)'};opacity:${entry.item ? '1' : '0.34'};`}
              ></span>
              <div
                class="pointer-events-none absolute bottom-full left-1/2 z-20 mb-2 hidden w-max max-w-[16rem] -translate-x-1/2 rounded-md border border-(--color-line-med) bg-(--color-bg-raised) px-3 py-2 text-xs leading-5 text-(--color-fg-2) shadow-[0_10px_30px_rgba(0,0,0,0.18)] group-hover:block"
              >
                {#if entry.item}
                  <p class={`font-medium ${resolveSiteCheckTextClass(entry.item.result)}`}>
                    {formatSiteCheckRegionLabel(entry.item.region)} · {formatSiteCheckResultLabel(
                      entry.item.result,
                    )}
                  </p>
                  {#each buildSiteCheckFacts(entry.item) as fact (`${fact.label}:${fact.value}`)}
                    <p>{fact.label}：{fact.value}</p>
                  {/each}
                  {#if entry.item.message}
                    <p class="max-w-56 text-(--color-fg-3)">
                      {entry.item.message}
                    </p>
                  {/if}
                {:else}
                  <p class="font-medium text-(--color-fg)">未检测</p>
                  <p>当前还没有可展示的站点检测记录。</p>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </div>
    </div>
  </div>
</div>

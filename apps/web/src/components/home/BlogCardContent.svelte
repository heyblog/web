<script lang="ts">
  import { IconExternalLink } from '@tabler/icons-svelte';
  import type { Attachment } from 'svelte/attachments';

  import { formatSiteUpdatedAt, type HomeSiteCard } from '@/application/home/home.shared';

  import {
    type BlogCardPlannedFields,
    type BlogCardTag,
    type BlogCardTagTone,
    createBlogCardTags,
    resolveVisibleTagCount,
  } from './blog-card-layout.shared';
  import BlogCardFooter from './BlogCardFooter.svelte';
  import BlogCardPlannedMetrics from './BlogCardPlannedMetrics.svelte';

  import './blog-card-content.css';

  type Props = {
    readonly site: HomeSiteCard;
    readonly expanded: boolean;
    readonly truncated: boolean;
    readonly expandedShell: boolean;
    readonly titleId: string;
    readonly descriptionId: string;
    readonly plannedFields?: BlogCardPlannedFields;
  };

  const accessScopeLabels = {
    ALL: '全球可访问',
    CN_ONLY: '仅中国大陆可访问',
    GLOBAL_ONLY: '仅海外可访问',
  } as const satisfies Record<HomeSiteCard['accessScope'], string>;
  const tagClasses = {
    warning:
      'inline-flex min-h-5 shrink-0 items-center rounded-sm bg-warning-bg px-2 text-xs font-medium text-warning-fg',
    primary:
      'inline-flex min-h-5 shrink-0 items-center rounded-sm bg-tint px-2 text-xs font-medium text-tint-fg',
    secondary:
      'inline-flex min-h-5 shrink-0 items-center rounded-sm bg-subtle px-2 text-xs font-medium text-fg-muted',
  } as const satisfies Record<BlogCardTagTone, string>;
  let { site, expanded, truncated, expandedShell, titleId, descriptionId, plannedFields }: Props =
    $props();
  let visibleTagCount = $state(Number.POSITIVE_INFINITY);
  let summaryExpandedHeight = $state(24);
  let tagsExpandedHeight = $state(20);
  const orderedTags = $derived(createBlogCardTags(site));
  const visibleTags = $derived(truncated ? orderedTags.slice(0, visibleTagCount) : orderedTags);
  const hiddenTagCount = $derived(truncated ? orderedTags.length - visibleTagCount : 0);
  const tagCounterValues = $derived(orderedTags.map((_tag, index) => index + 1));
  const summary = $derived(site.summary.trim() || '该博客已被 HeyBlog 收录，暂无博客简介。');

  function measureSummary(): Attachment<HTMLParagraphElement> {
    return (node) => {
      const measure = () => {
        summaryExpandedHeight = Math.max(24, Math.ceil(node.getBoundingClientRect().height));
      };
      const resizeObserver = new ResizeObserver(measure);
      resizeObserver.observe(node);
      measure();
      return () => resizeObserver.disconnect();
    };
  }

  function measureTagLayout(tags: readonly BlogCardTag[]): Attachment<HTMLDivElement> {
    return (node) => {
      let animationFrame = 0;
      const tagElements = [...node.querySelectorAll<HTMLElement>('[data-tag-measure]')];
      const counterElements = [...node.querySelectorAll<HTMLElement>('[data-count-measure]')];
      const fullTagList = node.querySelector<HTMLElement>('[data-full-tag-measure]');
      const measure = () => {
        cancelAnimationFrame(animationFrame);
        animationFrame = requestAnimationFrame(() => {
          const counterWidths = Array.from<number>({ length: tags.length + 1 }).fill(0);
          for (const element of counterElements) {
            const count = Number(element.dataset.countMeasure);
            if (Number.isInteger(count) && count > 0) {
              counterWidths[count] = element.getBoundingClientRect().width;
            }
          }
          visibleTagCount = resolveVisibleTagCount({
            containerWidth: node.clientWidth,
            tagWidths: tagElements.map((element) => element.getBoundingClientRect().width),
            counterWidths,
            gap: Number.parseFloat(getComputedStyle(fullTagList ?? node).columnGap) || 0,
            maxRows: 1,
          });
          tagsExpandedHeight = Math.max(
            20,
            Math.ceil(fullTagList?.getBoundingClientRect().height ?? 20),
          );
        });
      };
      const resizeObserver = new ResizeObserver(measure);
      resizeObserver.observe(node);
      if (fullTagList) resizeObserver.observe(fullTagList);
      for (const element of [...tagElements, ...counterElements]) resizeObserver.observe(element);
      measure();
      return () => {
        cancelAnimationFrame(animationFrame);
        resizeObserver.disconnect();
      };
    };
  }
</script>

<div class="relative min-w-0" data-blog-card-content data-expanded={expanded}>
  <header class="relative min-h-12">
    <div class="min-w-0">
      <h3 class="min-w-0 text-xl/snug font-semibold" id={titleId}>
        <a
          class="pointer-events-auto relative z-10 flex max-w-full min-w-0 items-center gap-1.5 rounded-sm outline-none hover:text-tint-fg focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-offset-2 focus-visible:ring-offset-surface"
          href={site.homepageUrl}
          target="_blank"
          rel="noreferrer"
        >
          <span class="truncate">{site.name}</span>
          <IconExternalLink class="shrink-0" aria-hidden="true" size={15} stroke={1.8} />
        </a>
      </h3>
      <p class="mt-1 truncate font-mono text-xs text-fg-muted">
        {site.host}
      </p>
    </div>
  </header>

  <div
    class="relative mt-2 overflow-hidden"
    data-card-section="summary"
    data-expanded={expanded}
    style:--expanded-height={`${summaryExpandedHeight}px`}
  >
    <p
      class={['text-sm/6 wrap-break-word break-keep text-fg-muted', truncated && 'line-clamp-1']}
      id={descriptionId}
    >
      {summary}
    </p>
    <p
      class="invisible absolute inset-x-0 top-0 text-sm/6 wrap-break-word break-keep text-fg-muted"
      aria-hidden="true"
      {@attach measureSummary()}
    >
      {summary}
    </p>
  </div>

  <div
    class="relative mt-2 overflow-hidden"
    data-card-section="tags"
    data-expanded={expanded}
    style:--expanded-height={`${tagsExpandedHeight}px`}
  >
    <div class="flex flex-wrap gap-1.5">
      {#each visibleTags as tag (tag.key)}
        <span class={tagClasses[tag.tone]}>{tag.label}</span>
      {/each}
      {#if hiddenTagCount > 0}
        <span class={tagClasses.secondary} aria-label={`另有 ${hiddenTagCount} 个标签`}
          >+{hiddenTagCount}</span
        >
      {/if}
    </div>
    <div
      class="invisible absolute inset-x-0 top-0"
      aria-hidden="true"
      {@attach measureTagLayout(orderedTags)}
    >
      <div class="flex flex-wrap gap-1.5" data-full-tag-measure>
        {#each orderedTags as tag (tag.key)}
          <span class={tagClasses[tag.tone]} data-tag-measure>{tag.label}</span>
        {/each}
      </div>
      <div class="absolute inset-x-0 top-0 flex gap-1.5 overflow-hidden">
        {#each tagCounterValues as count (count)}
          <span class={tagClasses.secondary} data-count-measure={count}>+{count}</span>
        {/each}
      </div>
    </div>
  </div>

  <BlogCardPlannedMetrics {plannedFields} />

  <div data-card-section="details" data-expanded={expanded} aria-hidden={!expanded}>
    <div class="overflow-hidden">
      <dl class="mt-3 grid gap-3 border-y border-line py-3 text-xs sm:grid-cols-2">
        <div>
          <dt class="text-fg-muted">访问范围</dt>
          <dd class="mt-1 font-medium">{accessScopeLabels[site.accessScope]}</dd>
        </div>
        <div>
          <dt class="text-fg-muted">信息更新时间</dt>
          <dd class="mt-1 font-medium">{formatSiteUpdatedAt(site.updatedAt)}</dd>
        </div>
      </dl>
    </div>
  </div>

  <BlogCardFooter {site} expanded={expandedShell} feedback={plannedFields?.feedback} />
</div>

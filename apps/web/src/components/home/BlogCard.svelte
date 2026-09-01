<script lang="ts">
  import type { HomeSiteCard } from '@/application/home/home.shared';

  import type { BlogCardPlannedFields } from './blog-card-layout.shared';
  import BlogCardContent from './BlogCardContent.svelte';
  import BlogCardExpanded from './BlogCardExpanded.svelte';
  import BlogCardStateStrip from './BlogCardStateStrip.svelte';

  type Props = {
    readonly site: HomeSiteCard;
    readonly expanded: boolean;
    readonly onExpand: () => void;
    readonly onClose: () => void;
    readonly plannedFields?: BlogCardPlannedFields;
  };

  let { site, expanded, onExpand, onClose, plannedFields }: Props = $props();
  let cardElement = $state<HTMLElement | null>(null);
  let openerElement = $state<HTMLButtonElement | null>(null);
  const dialogId = $derived(`expanded-card-dialog-${site.shortId}`);
  const titleId = $derived(`compact-card-title-${site.shortId}`);
  const descriptionId = $derived(`compact-card-description-${site.shortId}`);
</script>

<div class="contents">
  <article
    bind:this={cardElement}
    class="group/card relative isolate h-64 w-full max-w-full min-w-0 overflow-hidden rounded-md border border-line bg-surface shadow-2xs transition-[transform,border-color] duration-(--motion-base) ease-standard motion-reduce:transform-none motion-reduce:transition-colors [@media(hover:hover)_and_(pointer:fine)]:hover:-translate-y-px [@media(hover:hover)_and_(pointer:fine)]:hover:border-line-strong [@media(hover:hover)_and_(pointer:fine)]:hover:shadow-xs"
    data-blog-card
    data-site-id={site.shortId}
  >
    <button
      bind:this={openerElement}
      class={[
        'group/state absolute inset-0 z-0 cursor-pointer rounded-md outline-none focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-inset',
        expanded && 'invisible',
      ]}
      type="button"
      aria-label={`展开 ${site.name} 的博客卡片`}
      aria-expanded={expanded}
      aria-haspopup="dialog"
      aria-controls={dialogId}
      onclick={onExpand}
    >
      <span
        class="absolute inset-x-0 bottom-0 flex h-11 items-center justify-center overflow-hidden border-t border-line text-fg-muted sm:h-10"
        aria-hidden="true"
      >
        <BlogCardStateStrip expanded={false} />
      </span>
    </button>

    <div
      class={[
        'pointer-events-none relative z-10 h-[calc(100%-2.75rem)] overflow-hidden px-6 pt-6 sm:h-[calc(100%-2.5rem)]',
        expanded && 'invisible',
      ]}
    >
      <BlogCardContent
        {site}
        expanded={false}
        truncated={true}
        expandedShell={false}
        {titleId}
        {descriptionId}
        {plannedFields}
      />
    </div>
  </article>

  <BlogCardExpanded
    {site}
    open={expanded}
    {onClose}
    {plannedFields}
    sourceElement={cardElement}
    {openerElement}
    {dialogId}
  />
</div>

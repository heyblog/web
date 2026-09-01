<script lang="ts">
  import type { Snippet } from 'svelte';

  type Props = {
    readonly label: string;
    readonly accessibleLabel?: string;
    readonly className?: string;
    readonly children: Snippet;
  };

  let { label, accessibleLabel = label, className = '', children }: Props = $props();
</script>

<span
  class={`pointer-events-auto relative inline-flex min-w-0 ${className}`}
  data-blog-card-hint
  role="img"
  aria-label={accessibleLabel}
>
  {@render children()}
  <span
    class="absolute bottom-[calc(100%+0.125rem)] left-1/2 z-30 -translate-x-1/2 rounded-sm border border-line-strong bg-surface px-2 py-1 text-xs font-medium whitespace-nowrap text-fg shadow-sm"
    data-blog-card-hint-tooltip
    aria-hidden="true"
  >
    {label}
  </span>
</span>

<style>
  [data-blog-card-hint-tooltip] {
    visibility: hidden;
    opacity: 0;
    transform: translate(-50%, 2px);
    transition:
      opacity var(--motion-fast) var(--ease-standard),
      transform var(--motion-fast) var(--ease-standard),
      visibility 0s linear var(--motion-fast);
  }

  @media (hover: hover) and (pointer: fine) {
    [data-blog-card-hint]:hover [data-blog-card-hint-tooltip] {
      visibility: visible;
      opacity: 1;
      transform: translate(-50%, 0);
      transition-delay: 80ms;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    [data-blog-card-hint-tooltip] {
      transform: translate(-50%, 0);
      transition-property: opacity, visibility;
    }
  }
</style>

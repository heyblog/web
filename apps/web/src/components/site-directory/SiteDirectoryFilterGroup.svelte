<script lang="ts">
  import { IconChevronDown } from '@tabler/icons-svelte';
  import type { Snippet } from 'svelte';

  type Props = {
    readonly label: string;
    readonly count?: number;
    readonly expanded?: boolean;
    readonly children: Snippet;
  };
  let { label, count = 0, expanded = false, children }: Props = $props();
</script>

<details class="group/filter border-b border-line last:border-b-0" open={expanded}>
  <summary
    class="flex min-h-11 cursor-pointer list-none items-center gap-2 rounded-sm p-2 text-sm font-medium transition-colors duration-(--motion-fast) hover:bg-subtle sm:min-h-10 [&::-webkit-details-marker]:hidden"
  >
    <span class="min-w-0 flex-1 wrap-anywhere">{label}</span>
    {#if count > 0}
      <span class="shrink-0 rounded-sm bg-tint px-1.5 text-xs text-tint-fg">{count}</span>
    {/if}
    <IconChevronDown
      class="shrink-0 transition-transform duration-(--motion-base) group-open/filter:rotate-180 motion-reduce:transition-none"
      size={16}
      stroke={1.8}
      aria-hidden="true"
    />
  </summary>
  <div class="grid min-w-0 gap-2 px-2 pt-1 pb-3">
    {@render children()}
  </div>
</details>

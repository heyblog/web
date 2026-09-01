<script lang="ts">
  import { IconChevronDown, IconChevronUp } from '@tabler/icons-svelte';

  type Props = {
    readonly expanded: boolean;
  };

  let { expanded }: Props = $props();
</script>

<span
  class="relative flex size-full items-center justify-center overflow-hidden text-fg-muted"
  aria-hidden="true"
>
  <span
    class="absolute inset-0 origin-bottom scale-y-0 bg-linear-to-t from-tint to-transparent transition-transform duration-(--motion-base) ease-standard group-hover/card:scale-y-100 group-hover/state:scale-y-100 group-focus-visible/state:scale-y-100 motion-reduce:transition-none"
  ></span>
  {#if expanded}
    <IconChevronUp
      class="relative z-10 transition-colors duration-(--motion-fast) ease-standard group-hover/card:text-tint-fg group-hover/state:text-tint-fg group-focus-visible/state:text-tint-fg"
      size={17}
      stroke={1.8}
    />
  {:else}
    <IconChevronDown
      class="relative z-10 transition-colors duration-(--motion-fast) ease-standard group-hover/card:text-tint-fg group-hover/state:text-tint-fg group-focus-visible/state:text-tint-fg"
      size={17}
      stroke={1.8}
    />
  {/if}
</span>

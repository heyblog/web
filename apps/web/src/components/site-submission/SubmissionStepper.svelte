<script lang="ts">
  interface Props {
    labels: readonly string[];
    current: number;
    furthest: number;
    onchange: (step: number) => void;
  }
  let { labels, current, furthest, onchange }: Props = $props();
</script>

<nav aria-label="申请步骤">
  <ol class={['grid gap-2', labels.length === 4 ? 'sm:grid-cols-4' : 'sm:grid-cols-2']}>
    {#each labels as label, index (label)}
      <li>
        <button
          class="flex min-h-11 w-full items-center gap-2 rounded-sm border px-3 text-left text-sm transition-colors duration-(--motion-color) disabled:cursor-not-allowed"
          class:border-primary={index === current}
          class:bg-tint={index === current}
          class:text-tint-fg={index === current}
          class:border-line={index !== current}
          class:text-fg-muted={index !== current}
          type="button"
          disabled={index > furthest}
          aria-current={index === current ? 'step' : undefined}
          onclick={() => onchange(index)}
        >
          <span
            class="inline-flex size-6 shrink-0 items-center justify-center rounded-full border border-current text-xs"
            >{index + 1}</span
          >
          <span>{label}</span>
        </button>
      </li>
    {/each}
  </ol>
</nav>

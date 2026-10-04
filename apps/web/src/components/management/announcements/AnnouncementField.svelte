<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    label,
    validated = false,
    error,
    children,
  }: {
    label: string;
    error?: string;
    validated?: boolean;
    children: Snippet<[string, string | undefined]>;
  } = $props();
  const id = $props.id();
</script>

<div class="grid min-w-0 content-start gap-1.5">
  <label for={id} class="text-sm font-medium">{label}</label>
  {@render children(id, error)}
  {#if validated}<p id={`${id}-error`} class="min-h-10 text-sm/5 text-danger-fg">
      {error ?? ''}
    </p>{/if}
</div>

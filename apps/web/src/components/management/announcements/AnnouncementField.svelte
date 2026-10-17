<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    label,
    validated = false,
    error,
    help,
    children,
  }: {
    label: string;
    error?: string;
    help?: string;
    validated?: boolean;
    children: Snippet<[string, string | undefined]>;
  } = $props();
  const id = $props.id();
</script>

<div class="grid min-w-0 content-start gap-1.5">
  <label for={id} class="text-sm font-medium">{label}</label>
  {@render children(id, error)}
  {#if help}<p id={`${id}-help`} class="text-xs/normal text-fg-muted">{help}</p>{/if}
  {#if validated}<p id={`${id}-error`} class="min-h-10 text-sm/5 text-danger-fg">
      {error ?? ''}
    </p>{/if}
</div>

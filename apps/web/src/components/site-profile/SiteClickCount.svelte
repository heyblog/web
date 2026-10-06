<script lang="ts">
  import { IconPointer } from '@tabler/icons-svelte';

  let { count }: { readonly count: number } = $props();
  const formatted = $derived(new Intl.NumberFormat('zh-CN').format(count));
</script>

<span
  class="inline-flex min-w-0 items-center gap-1 text-xs text-fg-muted tabular-nums"
  aria-label={`点击 ${formatted} 次`}
>
  <IconPointer class="shrink-0" aria-hidden="true" size={14} stroke={1.8} />
  <span class="truncate">点击 {formatted}</span>
</span>

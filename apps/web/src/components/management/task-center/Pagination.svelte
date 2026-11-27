<script lang="ts">
  import { IconChevronLeft, IconChevronRight } from '@tabler/icons-svelte';

  import { buttonClass } from './styles';
  let {
    page,
    total,
    pageSize = 20,
    busy = false,
    onchange,
  }: {
    page: number;
    total: number;
    pageSize?: number;
    busy?: boolean;
    onchange: (page: number) => void;
  } = $props();
  const pages = $derived(Math.max(1, Math.ceil(total / pageSize)));
</script>

<nav class="flex flex-wrap items-center justify-between gap-3 text-sm" aria-label="分页">
  <span class="text-fg-muted">共 {total} 条 · 第 {page} / {pages} 页</span>
  <div class="flex gap-2">
    <button
      class={buttonClass}
      title="上一页"
      aria-label="上一页"
      disabled={busy || page <= 1}
      onclick={() => onchange(page - 1)}><IconChevronLeft size={16} /></button
    >
    <button
      class={buttonClass}
      title="下一页"
      aria-label="下一页"
      disabled={busy || page >= pages}
      onclick={() => onchange(page + 1)}><IconChevronRight size={16} /></button
    >
  </div>
</nav>

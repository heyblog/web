<script lang="ts">
  import { IconArrowRight, IconRefresh } from '@tabler/icons-svelte';
  import { onMount } from 'svelte';

  let { rerollHref, targetUrl }: { rerollHref: string; targetUrl: string } = $props();
  let remaining = $state(10);
  let paused = $state(false);
  let ready = $state(false);
  onMount(() => {
    ready = true;
    const timer = window.setInterval(() => {
      if (paused || document.hidden) return;
      if (remaining <= 1) {
        window.clearInterval(timer);
        window.location.assign(targetUrl);
        return;
      }
      remaining -= 1;
    }, 1_000);
    return () => window.clearInterval(timer);
  });
</script>

<div class="mt-6 border-t border-line pt-5">
  <p class="text-sm text-fg-muted">
    {ready ? (paused ? '已暂停自动跳转' : `${remaining} 秒后自动前往`) : '点击下方链接前往博客'}
  </p>
  <div class="mt-4 flex flex-wrap items-center gap-2">
    <a
      class="inline-flex min-h-11 items-center gap-1.5 rounded-md bg-primary px-4 text-sm font-semibold text-primary-fg transition-colors duration-(--motion-fast) hover:bg-primary-hover sm:min-h-10"
      href={targetUrl}
      rel="noreferrer"
      data-astro-prefetch="false"
    >
      立即前往 <IconArrowRight aria-hidden="true" size={16} stroke={1.8} />
    </a>
    <a
      class="inline-flex min-h-11 items-center gap-1.5 rounded-md px-4 text-sm font-medium text-fg-muted transition-colors duration-(--motion-fast) hover:bg-subtle hover:text-fg sm:min-h-10"
      href={rerollHref}
      data-astro-prefetch="false"
    >
      <IconRefresh aria-hidden="true" size={16} stroke={1.8} />重新随机
    </a>
    {#if ready}
      <button
        class="inline-flex min-h-11 items-center rounded-md px-4 text-sm font-medium text-fg-muted transition-colors duration-(--motion-fast) hover:bg-subtle hover:text-fg sm:min-h-10"
        type="button"
        onclick={() => (paused = !paused)}>{paused ? '继续倒计时' : '暂停倒计时'}</button
      >
    {/if}
  </div>
  <p class="sr-only" aria-live="polite">{paused ? '自动跳转已暂停' : ''}</p>
</div>

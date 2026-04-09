<script lang="ts">
  import { onMount } from 'svelte';
  import { flip } from 'svelte/animate';
  import { cubicOut } from 'svelte/easing';
  import { SvelteMap } from 'svelte/reactivity';
  import { fly } from 'svelte/transition';

  import {
    SNACKBAR_CLOSE_EVENT,
    SNACKBAR_OPEN_EVENT,
    type SnackbarPayload,
    type SnackbarTone,
    triggerSnackbarAction,
  } from '../browser/snackbar.service';

  interface SnackbarItem extends Required<Omit<SnackbarPayload, 'durationMs' | 'onAction'>> {
    durationMs: number | null;
  }

  const toneClassMap: Record<SnackbarTone, string> = {
    info: 'text-[color:var(--color-info)]',
    success: 'text-[color:var(--color-ok)]',
    warning: 'text-[color:var(--color-warn)]',
    error: 'text-[color:var(--color-fail)]',
  };

  const progressColorMap: Record<SnackbarTone, string> = {
    info: 'var(--color-info-dot)',
    success: 'var(--color-ok-dot)',
    warning: 'var(--color-warn-dot)',
    error: 'var(--color-fail-dot)',
  };

  let items: SnackbarItem[] = [];
  const timers = new SvelteMap<string, ReturnType<typeof setTimeout>>();
  const listMotion = { duration: 180, easing: cubicOut };
  const enterMotion = { y: 12, duration: 220, opacity: 0.18, easing: cubicOut };
  const exitMotion = { y: 8, duration: 160, opacity: 0.12, easing: cubicOut };
  const transitionHelpers = [flip, fly];
  void transitionHelpers;

  const clearTimer = (id: string) => {
    const timer = timers.get(id);

    if (timer) {
      window.clearTimeout(timer);
      timers.delete(id);
    }
  };

  const closeItem = (id: string) => {
    clearTimer(id);
    items = items.filter((item) => item.id !== id);
  };

  const scheduleClose = (item: SnackbarItem) => {
    clearTimer(item.id);

    if (typeof item.durationMs !== 'number' || item.durationMs <= 0) {
      return;
    }

    timers.set(
      item.id,
      setTimeout(() => {
        closeItem(item.id);
      }, item.durationMs),
    );
  };

  async function runAction(item: SnackbarItem) {
    await triggerSnackbarAction(item.id);
    closeItem(item.id);
  }

  onMount(() => {
    const handleOpen = (event: Event) => {
      const payload = (event as CustomEvent<SnackbarItem>).detail;
      items = [payload, ...items.filter((item) => item.id !== payload.id)].slice(0, 2);
      scheduleClose(payload);
    };

    const handleClose = (event: Event) => {
      const id = (event as CustomEvent<{ id?: string }>).detail?.id?.trim();

      if (!id) {
        const latest = items[0];

        if (latest) {
          closeItem(latest.id);
        }
        return;
      }

      closeItem(id);
    };

    window.addEventListener(SNACKBAR_OPEN_EVENT, handleOpen as EventListener);
    window.addEventListener(SNACKBAR_CLOSE_EVENT, handleClose as EventListener);

    return () => {
      window.removeEventListener(SNACKBAR_OPEN_EVENT, handleOpen as EventListener);
      window.removeEventListener(SNACKBAR_CLOSE_EVENT, handleClose as EventListener);
      timers.forEach((timer) => window.clearTimeout(timer));
      timers.clear();
    };
  });
</script>

<section
  class="pointer-events-none fixed inset-x-0 bottom-4 z-90 flex justify-center px-3 sm:bottom-5 sm:px-6"
>
  <div class="flex w-full max-w-lg flex-col gap-3">
    {#each items as item (item.id)}
      <article
        animate:flip={listMotion}
        in:fly={enterMotion}
        out:fly={exitMotion}
        class="pointer-events-auto relative overflow-hidden rounded-[5px] border border-(--color-line-med) bg-(--color-bg-raised) p-4 shadow-[0_18px_40px_rgba(28,25,23,0.14)] dark:shadow-[0_18px_40px_rgba(0,0,0,0.34)]"
      >
        <div class="grid gap-2">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              {#if item.title}
                <p class={`text-sm font-medium ${toneClassMap[item.tone]}`}>{item.title}</p>
              {/if}
            </div>
            <div class="flex shrink-0 items-center gap-2">
              {#if item.actionLabel}
                <button
                  class={`rounded-sm px-3 py-1.5 text-sm transition hover:bg-[color-mix(in_srgb,var(--color-bg)_76%,transparent)] hover:text-(--color-fg) ${toneClassMap[item.tone]}`}
                  type="button"
                  onclick={() => runAction(item)}
                >
                  {item.actionLabel}
                </button>
              {/if}
              <button
                class="inline-flex h-7 w-7 items-center justify-center rounded-sm text-(--color-fg-3) transition hover:bg-[color-mix(in_srgb,var(--color-bg)_76%,transparent)] hover:text-(--color-fg)"
                type="button"
                onclick={() => closeItem(item.id)}
                aria-label="关闭提示"
              >
                ×
              </button>
            </div>
          </div>
          <p class="text-sm leading-6 text-(--color-fg-2)">{item.message}</p>
        </div>
        {#if typeof item.durationMs === 'number' && item.durationMs > 0}
          <div
            class="pointer-events-none absolute inset-x-0 bottom-0 h-0.75 bg-[color-mix(in_srgb,var(--color-line)_38%,transparent)]"
          >
            <span
              class="snackbar-progress-bar block h-full origin-left"
              style:background-color={progressColorMap[item.tone]}
              style:animation-duration={`${item.durationMs}ms`}
            ></span>
          </div>
        {/if}
      </article>
    {/each}
  </div>
</section>

<style>
  .snackbar-progress-bar {
    animation-name: snackbar-progress;
    animation-timing-function: linear;
    animation-fill-mode: forwards;
  }

  @keyframes snackbar-progress {
    from {
      transform: scaleX(1);
    }

    to {
      transform: scaleX(0);
    }
  }
</style>

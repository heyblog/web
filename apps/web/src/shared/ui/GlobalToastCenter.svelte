<script lang="ts">
  import { onMount } from 'svelte';
  import { flip } from 'svelte/animate';
  import { cubicOut } from 'svelte/easing';
  import { SvelteMap } from 'svelte/reactivity';
  import { fly } from 'svelte/transition';

  import {
    TOAST_CLOSE_EVENT,
    TOAST_OPEN_EVENT,
    type ToastPayload,
    type ToastTone,
  } from '../browser/toast.service';

  interface ToastItem extends Required<Omit<ToastPayload, 'durationMs'>> {
    durationMs: number | null;
  }

  const toneClassMap: Record<ToastTone, string> = {
    info: 'text-[color:var(--color-info)]',
    success: 'text-[color:var(--color-ok)]',
    warning: 'text-[color:var(--color-warn)]',
    error: 'text-[color:var(--color-fail)]',
  };

  const borderColorMap: Record<ToastTone, string> = {
    info: 'color-mix(in srgb, var(--color-info) 34%, var(--color-line-med))',
    success: 'color-mix(in srgb, var(--color-ok) 34%, var(--color-line-med))',
    warning: 'color-mix(in srgb, var(--color-warn) 34%, var(--color-line-med))',
    error: 'color-mix(in srgb, var(--color-fail) 34%, var(--color-line-med))',
  };

  const progressColorMap: Record<ToastTone, string> = {
    info: 'var(--color-info-dot)',
    success: 'var(--color-ok-dot)',
    warning: 'var(--color-warn-dot)',
    error: 'var(--color-fail-dot)',
  };

  let items: ToastItem[] = [];
  const timers = new SvelteMap<string, ReturnType<typeof setTimeout>>();
  const listMotion = { duration: 180, easing: cubicOut };
  const enterMotion = { y: -12, duration: 220, opacity: 0.18, easing: cubicOut };
  const exitMotion = { y: -8, duration: 160, opacity: 0.12, easing: cubicOut };
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

  const scheduleClose = (item: ToastItem) => {
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

  onMount(() => {
    const handleOpen = (event: Event) => {
      const payload = (event as CustomEvent<ToastItem>).detail;
      items = [payload, ...items.filter((item) => item.id !== payload.id)].slice(0, 4);
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

    window.addEventListener(TOAST_OPEN_EVENT, handleOpen as EventListener);
    window.addEventListener(TOAST_CLOSE_EVENT, handleClose as EventListener);

    window.ZhblogsToast = {
      close: (id?: string) => {
        const target = id?.trim();
        if (target) {
          closeItem(target);
        } else if (items[0]) {
          closeItem(items[0].id);
        }
      },
      open: (payload) => {
        const normalized = {
          id: payload.id?.trim() || `toast-${Date.now()}`,
          title: payload.title.trim() || '提示',
          message: payload.message ?? '',
          tone: payload.tone ?? 'info',
          durationMs: payload.durationMs === undefined ? 4200 : payload.durationMs,
        } satisfies ToastItem;
        items = [normalized, ...items.filter((item) => item.id !== normalized.id)].slice(0, 4);
        scheduleClose(normalized);
        return normalized.id;
      },
    };

    return () => {
      window.removeEventListener(TOAST_OPEN_EVENT, handleOpen as EventListener);
      window.removeEventListener(TOAST_CLOSE_EVENT, handleClose as EventListener);
      timers.forEach((timer) => window.clearTimeout(timer));
      timers.clear();
    };
  });
</script>

<section
  class="pointer-events-none fixed inset-x-0 top-4 z-90 flex justify-center px-3 sm:top-5 sm:px-6"
>
  <div class="flex w-full max-w-100 flex-col gap-3">
    {#each items as item (item.id)}
      <article
        animate:flip={listMotion}
        in:fly={enterMotion}
        out:fly={exitMotion}
        class="pointer-events-auto relative overflow-hidden rounded-md border bg-(--color-bg-raised) p-4 shadow-[0_18px_40px_rgba(28,25,23,0.14)] transition dark:shadow-[0_18px_40px_rgba(0,0,0,0.34)]"
        style:border-color={borderColorMap[item.tone]}
        role="status"
      >
        <div class="grid gap-2">
          <div class="flex items-start justify-between gap-3">
            <p class={`min-w-0 flex-1 text-sm font-medium ${toneClassMap[item.tone]}`}>
              {item.title}
            </p>
            <button
              class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-sm text-(--color-fg-3) transition hover:bg-[color-mix(in_srgb,var(--color-bg)_76%,transparent)] hover:text-(--color-fg)"
              type="button"
              onclick={() => closeItem(item.id)}
              aria-label="关闭提示"
            >
              ×
            </button>
          </div>
        </div>
        {#if item.message}
          <p class="text-sm leading-6 text-(--color-fg-2)">{item.message}</p>
        {/if}
        {#if typeof item.durationMs === 'number' && item.durationMs > 0}
          <div
            class="pointer-events-none absolute inset-x-0 bottom-0 h-0.75 bg-[color-mix(in_srgb,var(--color-line)_38%,transparent)]"
          >
            <span
              class="toast-progress-bar block h-full origin-left"
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
  .toast-progress-bar {
    animation-name: toast-progress;
    animation-timing-function: linear;
    animation-fill-mode: forwards;
  }

  @keyframes toast-progress {
    from {
      transform: scaleX(1);
    }

    to {
      transform: scaleX(0);
    }
  }
</style>

<script lang="ts">
  import { IconX } from '@tabler/icons-svelte';
  import type { Snippet } from 'svelte';
  import { fade } from 'svelte/transition';

  import { buttonClass } from './api-keys.styles';

  interface Props {
    title: string;
    drawer: boolean;
    busy: boolean;
    onrequestclose: () => void;
    children: Snippet;
  }
  let { title, drawer, busy, onrequestclose, children }: Props = $props();
  let heading: HTMLHeadingElement | undefined = $state();
  $effect(() => {
    if (title) heading?.focus();
  });

  function attachDialog(element: HTMLDialogElement) {
    const previousFocus = document.activeElement;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    element.showModal();
    return () => {
      element.close();
      document.body.style.overflow = previousOverflow;
      if (previousFocus instanceof HTMLElement && previousFocus.isConnected) previousFocus.focus();
    };
  }

  function backdrop(event: MouseEvent): void {
    if (!(event.currentTarget instanceof HTMLDialogElement) || event.target !== event.currentTarget)
      return;
    const bounds = event.currentTarget.getBoundingClientRect();
    if (
      event.clientX < bounds.left ||
      event.clientX > bounds.right ||
      event.clientY < bounds.top ||
      event.clientY > bounds.bottom
    )
      onrequestclose();
  }
</script>

<dialog
  out:fade={{ duration: 150 }}
  {@attach attachDialog}
  class={[
    'border border-line bg-surface p-0 text-fg shadow-md backdrop:bg-overlay',
    drawer
      ? 'fixed inset-y-0 right-0 left-auto m-0 h-dvh max-h-dvh w-full max-w-full sm:w-120'
      : 'm-auto max-h-[calc(100dvh-2rem)] w-[min(calc(100%-2rem),30rem)] rounded-md',
  ]}
  aria-labelledby="api-key-panel-title"
  onkeydown={(event) => {
    if (event.key !== 'Escape' || event.isComposing) return;
    // Repeated native dialog cancellation can become non-cancelable in Chromium.
    event.preventDefault();
    event.stopPropagation();
    if (!event.repeat) onrequestclose();
  }}
  oncancel={(event) => {
    event.preventDefault();
    onrequestclose();
  }}
  onclick={backdrop}
>
  <div class={['flex min-h-0 flex-col', drawer ? 'h-full' : 'max-h-[calc(100dvh-2rem)]']}>
    <header class="flex shrink-0 items-center justify-between gap-4 border-b border-line px-6 py-4">
      <h2
        class="min-w-0 text-xl font-semibold wrap-break-word"
        id="api-key-panel-title"
        tabindex="-1"
        bind:this={heading}
      >
        {title}
      </h2>
      <button
        class={buttonClass}
        type="button"
        aria-label="关闭面板"
        disabled={busy}
        onclick={onrequestclose}
      >
        <IconX size={18} stroke={1.8} aria-hidden="true" />
      </button>
    </header>
    <div class="flex min-h-0 flex-1 flex-col overflow-y-auto overscroll-contain">
      {@render children()}
    </div>
  </div>
</dialog>

<style>
  dialog[open] {
    animation: panel-enter var(--motion-base) var(--ease-out);
  }

  @keyframes panel-enter {
    from {
      opacity: 0;
      transform: translateY(8px);
    }

    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    dialog[open] {
      animation: none;
    }
  }
</style>

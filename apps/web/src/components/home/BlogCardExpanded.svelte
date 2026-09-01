<script lang="ts">
  import { tick } from 'svelte';
  import { prefersReducedMotion } from 'svelte/motion';

  import type { HomeSiteCard } from '@/application/home/home.shared';

  import {
    applyDialogLayout,
    captureSourceRect,
    findFocusableElements,
    readMotionDuration,
    sourceTransform,
  } from './blog-card-dialog.browser';
  import type { BlogCardPlannedFields, BlogCardSourceRect } from './blog-card-layout.shared';
  import BlogCardContent from './BlogCardContent.svelte';
  import BlogCardStateStrip from './BlogCardStateStrip.svelte';

  import './blog-card-expanded.css';

  type DialogPhase = 'closed' | 'measuring' | 'opening' | 'open' | 'closing';

  type Props = {
    readonly site: HomeSiteCard;
    readonly open: boolean;
    readonly onClose: () => void;
    readonly sourceElement: HTMLElement | null;
    readonly openerElement: HTMLButtonElement | null;
    readonly dialogId: string;
    readonly plannedFields?: BlogCardPlannedFields;
  };

  let { site, open, onClose, sourceElement, openerElement, dialogId, plannedFields }: Props =
    $props();
  let dialog: HTMLDialogElement;
  let phase = $state<DialogPhase>('closed');
  let contentExpanded = $state(false);
  let contentTruncated = $state(true);
  let activeAnimation: Animation | null = null;
  let dialogResizeObserver: ResizeObserver | null = null;
  let sequence = 0;
  let sourceRect: BlogCardSourceRect | null = null;
  const titleId = $derived(`expanded-card-title-${site.shortId}`);
  const descriptionId = $derived(`expanded-card-description-${site.shortId}`);

  function stopActiveAnimation() {
    if (!activeAnimation) return;
    activeAnimation.onfinish = null;
    activeAnimation.oncancel = null;
    activeAnimation.cancel();
    activeAnimation = null;
  }

  function observeDialogSize() {
    dialogResizeObserver?.disconnect();
    dialogResizeObserver = new ResizeObserver(handleResize);
    dialogResizeObserver.observe(dialog);
  }

  async function showDialog() {
    const currentSequence = ++sequence;
    sourceRect = captureSourceRect(sourceElement);
    if (!sourceRect) return;

    stopActiveAnimation();
    phase = 'measuring';
    contentExpanded = true;
    contentTruncated = false;
    dialog.showModal();
    observeDialogSize();
    await tick();
    if (currentSequence !== sequence || !open || !dialog.open || !sourceRect) return;

    applyDialogLayout(dialog, sourceRect);
    contentExpanded = false;
    contentTruncated = true;
    await tick();
    if (currentSequence !== sequence || !open || !dialog.open || !sourceRect) return;

    phase = 'opening';
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        if (currentSequence !== sequence || !open || !dialog.open || !sourceRect) return;
        dialog.focus({ preventScroll: true });
        contentExpanded = true;
        contentTruncated = false;
        if (prefersReducedMotion.current) {
          phase = 'open';
          return;
        }

        dialog.style.willChange = 'transform, opacity';
        const animation = dialog.animate(
          [
            { transform: sourceTransform(dialog, sourceRect), opacity: 0.92 },
            { transform: 'translate(0, 0) scale(1)', opacity: 1 },
          ],
          {
            duration: readMotionDuration(dialog, '--motion-base', 250),
            easing: 'cubic-bezier(0.22, 1, 0.36, 1)',
            fill: 'both',
          },
        );
        activeAnimation = animation;
        animation.onfinish = () => {
          if (activeAnimation !== animation) return;
          animation.cancel();
          activeAnimation = null;
          dialog.style.removeProperty('will-change');
          phase = 'open';
        };
      });
    });
  }

  function finishClose() {
    stopActiveAnimation();
    if (dialog.open) dialog.close();
  }

  function requestClose() {
    if (!dialog.open || phase === 'closing') return;
    sequence += 1;
    stopActiveAnimation();
    phase = 'closing';
    contentExpanded = false;
    contentTruncated = false;
    if (prefersReducedMotion.current || !sourceRect) {
      finishClose();
      return;
    }

    dialog.style.willChange = 'transform, opacity';
    const animation = dialog.animate(
      [
        { transform: 'translate(0, 0) scale(1)', opacity: 1 },
        { transform: sourceTransform(dialog, sourceRect), opacity: 0 },
      ],
      {
        duration: readMotionDuration(dialog, '--motion-fast', 150),
        easing: 'cubic-bezier(0.22, 1, 0.36, 1)',
        fill: 'both',
      },
    );
    activeAnimation = animation;
    animation.onfinish = finishClose;
  }

  function handleDialogClick(event: MouseEvent) {
    if (event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    const outside =
      event.clientX < bounds.left ||
      event.clientX > bounds.right ||
      event.clientY < bounds.top ||
      event.clientY > bounds.bottom;
    if (outside) requestClose();
  }

  function handleCancel(event: Event) {
    event.preventDefault();
    requestClose();
  }

  function handleDocumentKeydown(event: KeyboardEvent) {
    if (event.key !== 'Tab' || !dialog.open) return;
    const focusable = findFocusableElements(dialog);
    if (focusable.length === 0) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    const activeElement = document.activeElement;
    const activeIndex =
      activeElement instanceof HTMLElement ? focusable.indexOf(activeElement) : -1;
    const wrapsBackward = event.shiftKey && activeIndex === 0;
    const wrapsForward = !event.shiftKey && activeIndex === focusable.length - 1;
    if (activeIndex !== -1 && !wrapsBackward && !wrapsForward) return;
    event.preventDefault();
    (event.shiftKey ? last : first).focus();
  }

  function handleResize() {
    if (!dialog.open || phase === 'measuring' || phase === 'closing') return;
    sourceRect = captureSourceRect(sourceElement);
    if (!sourceRect) return;
    applyDialogLayout(dialog, sourceRect);
  }

  function handleClosed() {
    dialogResizeObserver?.disconnect();
    dialogResizeObserver = null;
    phase = 'closed';
    contentExpanded = false;
    contentTruncated = true;
    dialog.style.removeProperty('will-change');
    onClose();
    queueMicrotask(() => {
      if (openerElement?.isConnected) openerElement.focus({ preventScroll: true });
    });
  }

  $effect(() => {
    if (!dialog) return;
    if (open && !dialog.open) {
      void showDialog();
    } else if (!open && dialog.open) {
      requestClose();
    }
  });
</script>

<svelte:window onresize={handleResize} />
<svelte:document onkeydown={handleDocumentKeydown} />

<dialog
  bind:this={dialog}
  tabindex="-1"
  id={dialogId}
  data-blog-card-dialog
  class="fixed m-0 max-w-none overflow-y-auto overscroll-contain rounded-md border border-line-strong bg-surface p-0 text-fg shadow-md outline-none backdrop:bg-slate-950/60 dark:backdrop:bg-zinc-950/70"
  class:[--card-content-duration:var(--motion-fast)]={phase === 'closing'}
  data-phase={phase}
  aria-labelledby={titleId}
  aria-describedby={descriptionId}
  onclick={handleDialogClick}
  oncancel={handleCancel}
  onclose={handleClosed}
>
  <div class="px-6 pt-6 pb-2">
    <BlogCardContent
      {site}
      expanded={contentExpanded}
      truncated={contentTruncated}
      expandedShell={true}
      {titleId}
      {descriptionId}
      {plannedFields}
    />
  </div>
  <button
    class="group/state flex h-11 w-full items-center justify-center border-t border-line outline-none focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-inset sm:h-10"
    type="button"
    aria-label={`收起 ${site.name} 的博客卡片`}
    onclick={requestClose}
  >
    <BlogCardStateStrip expanded={true} />
  </button>
</dialog>

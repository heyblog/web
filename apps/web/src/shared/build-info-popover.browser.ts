import { resolvePopoverPosition } from '@/shared/build-info-position.shared';

const viewportPaddingRem = 1;
const popoverGapRem = 0.75;
const initializedPanels = new WeakSet<HTMLElement>();

function readViewportBounds() {
  const viewport = window.visualViewport;
  const left = viewport?.offsetLeft ?? 0;
  const top = viewport?.offsetTop ?? 0;
  const width = viewport?.width ?? window.innerWidth;
  const height = viewport?.height ?? window.innerHeight;

  return {
    left,
    top,
    right: left + width,
    bottom: top + height,
  };
}

function positionPopover(trigger: HTMLElement, panel: HTMLElement): void {
  const triggerRect = trigger.getBoundingClientRect();
  const panelRect = panel.getBoundingClientRect();
  const rootFontSize = Number.parseFloat(getComputedStyle(document.documentElement).fontSize);
  const position = resolvePopoverPosition({
    trigger: triggerRect,
    panel: panelRect,
    viewport: readViewportBounds(),
    padding: rootFontSize * viewportPaddingRem,
    gap: rootFontSize * popoverGapRem,
  });

  panel.dataset.popoverSide = position.side;
  panel.style.left = `${position.left}px`;
  panel.style.top = `${position.top}px`;
}

function setupPopover(panel: HTMLElement): void {
  const triggerId = panel.dataset.buildInfoTrigger;
  const trigger = triggerId ? document.getElementById(triggerId) : null;

  if (!(trigger instanceof HTMLElement) || initializedPanels.has(panel)) {
    return;
  }

  initializedPanels.add(panel);
  const lifecycle = new AbortController();
  const { signal } = lifecycle;
  let animationFrame = 0;

  const isOpen = () => panel.matches(':popover-open');
  const schedulePosition = () => {
    if (!isOpen() || animationFrame !== 0) {
      return;
    }

    animationFrame = window.requestAnimationFrame(() => {
      animationFrame = 0;

      if (isOpen()) {
        positionPopover(trigger, panel);
      }
    });
  };
  const syncOpenState = () => {
    const open = isOpen();
    trigger.setAttribute('aria-expanded', String(open));

    if (open) {
      positionPopover(trigger, panel);
      schedulePosition();
      return;
    }

    const activeElement = document.activeElement;

    if (activeElement instanceof Node && panel.contains(activeElement)) {
      trigger.focus({ preventScroll: true });
    }
  };
  const resizeObserver = new ResizeObserver(schedulePosition);

  panel.addEventListener('toggle', syncOpenState, { signal });
  window.addEventListener('resize', schedulePosition, { passive: true, signal });
  window.addEventListener('scroll', schedulePosition, { passive: true, signal });
  window.visualViewport?.addEventListener('resize', schedulePosition, { passive: true, signal });
  window.visualViewport?.addEventListener('scroll', schedulePosition, { passive: true, signal });
  resizeObserver.observe(trigger);
  resizeObserver.observe(panel);
  document.addEventListener('astro:before-swap', () => lifecycle.abort(), { once: true, signal });
  signal.addEventListener(
    'abort',
    () => {
      window.cancelAnimationFrame(animationFrame);
      resizeObserver.disconnect();
      initializedPanels.delete(panel);
    },
    { once: true },
  );
  syncOpenState();
}

export function initBuildInfoPopovers(): void {
  document.querySelectorAll<HTMLElement>('[data-build-info-popover]').forEach(setupPopover);
}

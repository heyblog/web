import {
  createFullscreenEscapeGuard,
  createGraphFullscreen,
  type GraphFullscreenMode,
} from './site-graph.fullscreen.ts';

export type { GraphFullscreenMode } from './site-graph.fullscreen.ts';

/** Locks are owned by the expanded boundary, not by either fullscreen mode. */
export function graphFullscreen(
  host: HTMLElement,
  change: (mode: GraphFullscreenMode) => void,
  notify: (message: string) => void,
) {
  const document = host.ownerDocument;
  let mode: GraphFullscreenMode = 'inline';
  let previous: Element | null = null;
  let overflow = '';
  let role: string | null = null;
  let modal: string | null = null;
  let scroll: readonly [number, number] = [0, 0];
  let focusVersion = 0;
  const escape = createFullscreenEscapeGuard();
  let escapeFrame: number | undefined;
  const inert = new Map<HTMLElement, boolean>();
  function lock(): void {
    previous = document.activeElement;
    scroll = [window.scrollX, window.scrollY];
    overflow = document.body.style.overflow;
    role = host.getAttribute('role');
    modal = host.getAttribute('aria-modal');
    document.body.style.overflow = 'hidden';
    let current: HTMLElement = host;
    while (current.parentElement) {
      for (const child of current.parentElement.children)
        if (child instanceof HTMLElement && child !== current) {
          inert.set(child, child.inert);
          child.inert = true;
        }
      current = current.parentElement;
      if (current === document.body) break;
    }
    host.setAttribute('role', 'dialog');
    host.setAttribute('aria-modal', 'true');
  }
  function unlock(): void {
    document.body.style.overflow = overflow;
    for (const [element, state] of inert) element.inert = state;
    inert.clear();
    if (role === null) host.removeAttribute('role');
    else host.setAttribute('role', role);
    if (modal === null) host.removeAttribute('aria-modal');
    else host.setAttribute('aria-modal', modal);
  }
  const controller = createGraphFullscreen({
    nativeSupported:
      typeof host.requestFullscreen === 'function' &&
      typeof document.exitFullscreen === 'function' &&
      document.fullscreenEnabled !== false,
    isNative: () => document.fullscreenElement === host,
    enterNative: () => host.requestFullscreen(),
    exitNative: () => document.exitFullscreen(),
    notify,
    change(next) {
      const entering = mode === 'inline' && next !== 'inline';
      const leaving = mode !== 'inline' && next === 'inline';
      if (entering) lock();
      if (leaving) unlock();
      mode = next;
      change(next);
      if (!entering && !leaving) return;
      const version = ++focusVersion;
      queueMicrotask(() => {
        if (version !== focusVersion) return;
        if (entering)
          host
            .querySelector<HTMLElement>('[data-graph-fullscreen]')
            ?.focus({ preventScroll: true });
        else {
          window.scrollTo({ left: scroll[0], top: scroll[1], behavior: 'instant' });
          if (previous instanceof HTMLElement && previous.isConnected)
            previous.focus({ preventScroll: true });
        }
      });
    },
  });
  const changed = () => {
    const wasNative = mode === 'native';
    controller.nativeChanged();
    if (!wasNative || mode === 'native') return;
    // Browsers may dispatch the fullscreen event before the Escape key event.
    escape.nativeExited();
    if (escapeFrame !== undefined) cancelAnimationFrame(escapeFrame);
    escapeFrame = requestAnimationFrame(() => {
      escape.release();
      escapeFrame = undefined;
    });
  };
  const keyboard = (event: KeyboardEvent) => {
    if (mode === 'inline' && document.fullscreenElement !== host) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      if (!event.repeat && escape.press()) void controller.escape();
      return;
    }
    if (event.key !== 'Tab') return;
    const elements = Array.from(
      host.querySelectorAll<HTMLElement>('button, a[href], input, select, summary, [tabindex="0"]'),
    ).filter((element) => !element.hasAttribute('disabled') && element.getClientRects().length > 0);
    const first = elements[0];
    const last = elements.at(-1);
    if (!host.contains(document.activeElement)) {
      event.preventDefault();
      (event.shiftKey ? last : first)?.focus();
    } else if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last?.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first?.focus();
    }
  };
  const released = (event: KeyboardEvent) => {
    if (event.key !== 'Escape') return;
    escape.release();
    if (escapeFrame !== undefined) cancelAnimationFrame(escapeFrame);
    escapeFrame = undefined;
  };
  document.addEventListener('fullscreenchange', changed);
  document.addEventListener('keydown', keyboard, true);
  document.addEventListener('keyup', released, true);
  return {
    togglePage: controller.togglePage,
    toggleNative: controller.toggleNative,
    close: controller.close,
    nativeSupported: controller.nativeSupported,
    destroy(): void {
      controller.destroy();
      document.removeEventListener('fullscreenchange', changed);
      document.removeEventListener('keydown', keyboard, true);
      document.removeEventListener('keyup', released, true);
      if (escapeFrame !== undefined) cancelAnimationFrame(escapeFrame);
    },
  };
}

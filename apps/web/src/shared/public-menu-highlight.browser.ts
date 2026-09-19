export interface PublicMenuHighlightController {
  hide: () => void;
  sync: () => void;
}

export function setupPublicMenuHighlight(
  menu: HTMLDetailsElement,
  signal: AbortSignal,
): PublicMenuHighlightController {
  const list = menu.querySelector<HTMLElement>('[data-public-menu-list]');
  const highlight = menu.querySelector<HTMLElement>('[data-public-menu-highlight]');

  if (!list || !highlight) {
    return { hide: () => undefined, sync: () => undefined };
  }

  const items = [...list.querySelectorAll<HTMLAnchorElement>('[data-public-menu-item]')];
  const activeItem = () =>
    items.find((item) => item.dataset.menuActive === 'true' && item.checkVisibility());
  const position = (item?: HTMLAnchorElement) => {
    if (!item || !item.checkVisibility()) {
      delete highlight.dataset.visible;
      delete highlight.dataset.active;
      return;
    }

    const offset = item.getBoundingClientRect().top - list.getBoundingClientRect().top;
    highlight.style.setProperty('--menu-highlight-offset', `${offset}px`);
    highlight.dataset.visible = 'true';
    highlight.dataset.active = String(item.dataset.menuActive === 'true');
  };
  const sync = () => window.requestAnimationFrame(() => position(activeItem()));
  const hide = () => {
    delete highlight.dataset.visible;
    delete highlight.dataset.active;
  };
  const itemFromEvent = (event: Event) =>
    event.target instanceof Element
      ? event.target.closest<HTMLAnchorElement>('[data-public-menu-item]')
      : null;

  list.addEventListener('pointerover', (event) => position(itemFromEvent(event) ?? undefined), {
    signal,
  });
  list.addEventListener('pointerleave', () => position(activeItem()), { signal });
  list.addEventListener('focusin', (event) => position(itemFromEvent(event) ?? undefined), {
    signal,
  });
  list.addEventListener(
    'focusout',
    (event) => {
      if (event.relatedTarget instanceof Node && list.contains(event.relatedTarget)) return;
      position(activeItem());
    },
    { signal },
  );

  return { hide, sync };
}

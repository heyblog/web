import { type AnimatedDetailsController, setupAnimatedDetails } from './animated-details.browser';
import { resolveBrandVisibility } from './public-header.shared';
import { setupPublicMenuHighlight } from './public-menu-highlight.browser';

const initializedHeaders = new WeakSet<HTMLElement>();

export function initPublicHeader(): void {
  document.querySelectorAll<HTMLElement>('[data-public-header]').forEach((header) => {
    if (initializedHeaders.has(header)) return;
    initializedHeaders.add(header);
    setupHeader(header);
  });
}

function setupHeader(header: HTMLElement): void {
  const lifecycle = new AbortController();
  const { signal } = lifecycle;
  const brand = header.querySelector<HTMLElement>('[data-public-brand]');
  const home = header.dataset.home === 'true';
  const hero = home ? document.querySelector<HTMLElement>('[data-home-hero]') : null;
  let observer: IntersectionObserver | undefined;
  let frame = 0;

  signal.addEventListener(
    'abort',
    () => {
      observer?.disconnect();
      window.cancelAnimationFrame(frame);
      initializedHeaders.delete(header);
    },
    { once: true },
  );
  const setBrandVisible = (visible: boolean) => {
    if (brand) {
      brand.dataset.visible = String(visible);
    }
  };
  const brandThreshold = () => header.offsetHeight + 12;
  const syncBrandVisibility = () => {
    setBrandVisible(
      resolveBrandVisibility({
        home,
        heroBottom: hero?.getBoundingClientRect().bottom ?? null,
        threshold: brandThreshold(),
      }),
    );
  };

  if (home && hero && 'IntersectionObserver' in window) {
    const observeHero = () => {
      observer?.disconnect();
      const threshold = brandThreshold();
      observer = new IntersectionObserver(
        ([entry]) => {
          setBrandVisible(
            resolveBrandVisibility({
              home,
              heroBottom: entry?.boundingClientRect.bottom ?? null,
              threshold,
            }),
          );
        },
        { rootMargin: `-${threshold}px 0px 0px`, threshold: 0 },
      );
      observer.observe(hero);
    };
    observeHero();
    window.addEventListener('resize', observeHero, { passive: true, signal });
  } else if (home) {
    const scheduleBrandSync = () => {
      if (frame !== 0) {
        return;
      }
      frame = window.requestAnimationFrame(() => {
        frame = 0;
        syncBrandVisibility();
      });
    };
    window.addEventListener('scroll', scheduleBrandSync, { passive: true, signal });
    window.addEventListener('resize', scheduleBrandSync, { passive: true, signal });
    syncBrandVisibility();
  } else {
    setBrandVisible(true);
  }
  const controllers = new Map<HTMLDetailsElement, AnimatedDetailsController>();
  const closeOthers = (except?: HTMLDetailsElement) => {
    controllers.forEach((controller, menu) => {
      if (menu !== except) controller.close();
    });
  };

  header.querySelectorAll<HTMLDetailsElement>('[data-public-menu]').forEach((menu) => {
    const panel = menu.querySelector<HTMLElement>('[data-public-menu-panel]');
    const trigger = menu.querySelector<HTMLElement>('[data-public-menu-trigger]');
    if (!panel || !trigger) return;
    const highlight = setupPublicMenuHighlight(menu, signal);
    const controller = setupAnimatedDetails(menu, {
      panel,
      onExpandedChange: (expanded) => {
        if (expanded) {
          closeOthers(menu);
          highlight.sync();
        } else {
          highlight.hide();
        }
      },
    });
    controllers.set(menu, controller);
    menu.addEventListener(
      'keydown',
      (event) => {
        if (!['ArrowDown', 'ArrowUp', 'Home', 'End', 'Escape'].includes(event.key)) return;
        if (event.key === 'Escape') {
          event.preventDefault();
          controller.close({ restoreFocus: true });
          return;
        }
        event.preventDefault();
        controller.open();
        const links = [...panel.querySelectorAll<HTMLAnchorElement>('a[href]')].filter(
          (link) => link.getClientRects().length > 0,
        );
        const current = links.findIndex((link) => link === document.activeElement);
        let next: number;
        switch (event.key) {
          case 'ArrowDown':
            next = (current + 1) % links.length;
            break;
          case 'ArrowUp':
            next = current <= 0 ? links.length - 1 : current - 1;
            break;
          case 'Home':
            next = 0;
            break;
          case 'End':
            next = links.length - 1;
            break;
          default:
            return;
        }
        links[next]?.focus();
      },
      { signal },
    );
    menu.addEventListener(
      'focusout',
      (event) => {
        if (event.relatedTarget instanceof Node && !menu.contains(event.relatedTarget))
          controller.close();
      },
      { signal },
    );
  });

  document.addEventListener(
    'click',
    (event) => {
      if (!(event.target instanceof Element)) return;
      const target = event.target;
      controllers.forEach((controller, menu) => {
        if (!menu.contains(target) || target.closest('a')) controller.close();
      });
    },
    { signal },
  );
  ['30rem', '48rem', '64rem', '80rem'].forEach((width) => {
    window.matchMedia(`(min-width: ${width})`).addEventListener(
      'change',
      () => {
        const focused = document.activeElement;
        controllers.forEach((controller) => controller.close({ immediate: true }));
        if (
          focused instanceof HTMLElement &&
          header.contains(focused) &&
          !focused.checkVisibility()
        ) {
          header.querySelector<HTMLElement>('a[href="/"]')?.focus();
        }
      },
      { signal },
    );
  });
  document.addEventListener(
    'astro:before-swap',
    () => {
      controllers.forEach((controller) => controller.close({ immediate: true }));
      lifecycle.abort();
      initializedHeaders.delete(header);
    },
    { once: true, signal },
  );
}

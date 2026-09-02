import {
  type AnimatedDetailsController,
  setupAnimatedDetails,
} from '@/shared/animated-details.browser';
import { resolveBrandVisibility } from '@/shared/public-header.shared';

const initializedHeaders = new WeakSet<HTMLElement>();
const menuControllers = new WeakMap<HTMLDetailsElement, AnimatedDetailsController>();

export function initPublicHeader(): void {
  document.querySelectorAll<HTMLElement>('[data-public-header]').forEach((header) => {
    if (initializedHeaders.has(header)) {
      return;
    }
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
  document.addEventListener('astro:before-swap', () => lifecycle.abort(), {
    once: true,
    signal,
  });

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

  header.querySelectorAll<HTMLDetailsElement>('[data-public-menu]').forEach((menu) => {
    const panel = menu.querySelector<HTMLElement>('[data-public-menu-panel]');
    const trigger = menu.querySelector<HTMLElement>('[data-public-menu-trigger]');

    if (!panel || !trigger) {
      return;
    }

    menuControllers.set(
      menu,
      setupAnimatedDetails(menu, {
        panel,
        onExpandedChange: (expanded) => {
          trigger.setAttribute('aria-label', expanded ? '关闭导航' : '打开导航');
        },
      }),
    );
  });

  const closeMenu = (menu: HTMLDetailsElement, restoreFocus = false, immediate = false) => {
    menuControllers.get(menu)?.close({ immediate, restoreFocus });
  };
  const desktopNavigation = window.matchMedia('(min-width: 64rem)');
  const closeMenusAtDesktop = () => {
    if (!desktopNavigation.matches) {
      return;
    }
    header
      .querySelectorAll<HTMLDetailsElement>('[data-public-mobile-menu][open]')
      .forEach((menu) => {
        closeMenu(menu, false, true);
      });
  };

  desktopNavigation.addEventListener('change', closeMenusAtDesktop, { signal });
  closeMenusAtDesktop();

  document.addEventListener(
    'click',
    (event) => {
      const target = event.target;

      if (!(target instanceof Element)) {
        return;
      }
      header.querySelectorAll<HTMLDetailsElement>('[data-public-menu][open]').forEach((menu) => {
        if (!menu.contains(target) || target.closest('a')) {
          closeMenu(menu);
        }
      });
    },
    { signal },
  );

  document.addEventListener(
    'keydown',
    (event) => {
      if (event.key !== 'Escape') {
        return;
      }
      header.querySelectorAll<HTMLDetailsElement>('[data-public-menu][open]').forEach((menu) => {
        closeMenu(menu, true);
      });
    },
    { signal },
  );
}

import { nextTabIndex } from '../../shared/tab-navigation.ts';

import { type ClaimView, claimViews, claimWorkspaceView } from './claims-view.ts';

export function setupClaimsWorkspace(root: HTMLElement): void {
  const navigation = root.querySelector<HTMLElement>('[data-claim-tabs]');
  const indicator = root.querySelector<HTMLElement>('[data-claim-indicator]');
  const entries = claimViews.flatMap((view) => {
    const tab = root.querySelector<HTMLAnchorElement>(`[data-claim-view="${view}"]`);
    const panel = root.querySelector<HTMLElement>(`[data-claim-panel="${view}"]`);
    return tab && panel ? [{ view, tab, panel }] : [];
  });
  if (!navigation || !indicator || entries.length !== claimViews.length) return;

  const lifecycle = new AbortController();
  const { signal } = lifecycle;
  let selected = claimWorkspaceView(new URLSearchParams(window.location.search));

  function render(): void {
    root.dataset.view = selected;
    navigation?.setAttribute('role', 'tablist');
    entries.forEach(({ view, tab, panel }) => {
      const active = view === selected;
      tab.dataset.selected = String(active);
      tab.removeAttribute('aria-current');
      tab.setAttribute('role', 'tab');
      tab.setAttribute('aria-selected', String(active));
      tab.setAttribute('aria-controls', panel.id);
      tab.tabIndex = active ? 0 : -1;
      panel.classList.toggle('hidden', !active);
      panel.setAttribute('role', 'tabpanel');
      panel.tabIndex = 0;
      panel.setAttribute('aria-labelledby', tab.id);
    });
    indicator?.classList.toggle('translate-x-full', selected === 'records');
  }

  function select(view: ClaimView, keyboard: boolean): void {
    selected = view;
    indicator?.classList.toggle('transition-transform', !keyboard);
    indicator?.classList.toggle('duration-(--motion-base)', !keyboard);
    render();
    // Claim creation replaces this URL with the resumable detail URL; keep it intact.
    const url = new URL(window.location.href);
    if (url.pathname === '/dashboard/claims' || url.pathname === '/dashboard/claims/') {
      url.searchParams.set('view', view);
      window.history.replaceState(window.history.state, '', url);
    }
  }

  entries.forEach(({ view, tab }, index) => {
    tab.addEventListener(
      'click',
      (event) => {
        if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey)
          return;
        event.preventDefault();
        select(view, event.detail === 0);
      },
      { signal },
    );
    tab.addEventListener(
      'keydown',
      (event) => {
        if (event.key === ' ') {
          event.preventDefault();
          select(view, true);
          return;
        }
        const next = nextTabIndex(event.key, index, entries.length);
        if (next === undefined) return;
        const entry = entries[next];
        if (!entry) return;
        event.preventDefault();
        select(entry.view, true);
        entry.tab.focus();
      },
      { signal },
    );
  });
  window.addEventListener(
    'popstate',
    () => {
      selected = claimWorkspaceView(new URLSearchParams(window.location.search));
      render();
    },
    { signal },
  );
  document.addEventListener('astro:before-swap', () => lifecycle.abort(), { once: true, signal });
  render();
}

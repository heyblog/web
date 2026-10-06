import { beaconImpressions, sendImpressions } from '../../api/site-metrics/site-metrics.browser.ts';
import type { SiteImpressionEvent } from '../../api/site-metrics/site-metrics.types.ts';

function documentEventId(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
  const variant = ((Number.parseInt(hex.slice(16, 18), 16) & 0x3f) | 0x80).toString(16);
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-4${hex.slice(13, 16)}-${variant}${hex.slice(18, 20)}-${hex.slice(20)}`;
}

const sessions = new WeakMap<Document, () => void>();

export function startSiteImpressions(document: Document): () => void {
  const existing = sessions.get(document);
  if (existing) return existing;
  const eventId = documentEventId();
  const seen = new Set<string>();
  const pending = new Map<string, SiteImpressionEvent>();
  const elements = new Map<
    Element,
    { id: string; ratio: number; timer?: ReturnType<typeof setTimeout> }
  >();
  let flushTimer: ReturnType<typeof setTimeout> | undefined;
  let sending = false;
  let suspended = false;

  function scheduleFlush(delay = 150): void {
    if (flushTimer !== undefined || document.hidden || suspended || pending.size === 0) return;
    flushTimer = setTimeout(() => {
      flushTimer = undefined;
      void flush();
    }, delay);
  }

  async function flush(): Promise<void> {
    if (sending || document.hidden || suspended || pending.size === 0) return;
    sending = true;
    const batch = [...pending.values()].slice(0, 100);
    const retry = await sendImpressions(batch);
    if (retry === 0) for (const event of batch) pending.delete(event.shortId);
    sending = false;
    scheduleFlush(retry || 150);
  }

  function update(element: Element): void {
    const state = elements.get(element);
    if (!state) return;
    if (state.timer !== undefined) {
      clearTimeout(state.timer);
      state.timer = undefined;
    }
    if (state.ratio < 0.5 || document.hidden || suspended || seen.has(state.id)) return;
    state.timer = setTimeout(() => {
      state.timer = undefined;
      if (
        !element.isConnected ||
        document.hidden ||
        suspended ||
        state.ratio < 0.5 ||
        seen.has(state.id)
      )
        return;
      seen.add(state.id);
      pending.set(state.id, { eventId, shortId: state.id });
      scheduleFlush();
    }, 1_000);
  }

  const observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        const state = elements.get(entry.target);
        if (!state) continue;
        state.ratio = entry.isIntersecting ? entry.intersectionRatio : 0;
        update(entry.target);
      }
    },
    { threshold: [0, 0.5] },
  );

  function discover(): void {
    for (const [element, state] of elements) {
      if (!element.isConnected || element.getAttribute('data-site-impression') !== state.id) {
        if (state.timer !== undefined) clearTimeout(state.timer);
        elements.delete(element);
        observer.unobserve(element);
      }
    }
    for (const element of document.querySelectorAll('[data-site-impression]')) {
      const id = element.getAttribute('data-site-impression') ?? '';
      if (!/^[0-9A-Za-z]{9}$/u.test(id) || elements.has(element)) continue;
      elements.set(element, { id, ratio: 0 });
      observer.observe(element);
    }
  }

  const mutations = new MutationObserver(discover);
  function refreshVisibility(): void {
    for (const [element, state] of elements) {
      state.ratio = 0;
      update(element);
      observer.unobserve(element);
      observer.observe(element);
    }
  }
  function resume(): void {
    suspended = false;
    discover();
    refreshVisibility();
    mutations.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ['data-site-impression'],
    });
    scheduleFlush();
  }
  function unload(): void {
    suspended = true;
    if (flushTimer !== undefined) clearTimeout(flushTimer);
    flushTimer = undefined;
    for (const element of elements.keys()) update(element);
    observer.disconnect();
    mutations.disconnect();
    const events = [...pending.values()];
    for (let i = 0; i < events.length; i += 100)
      beaconImpressions(events.slice(i, i + 100), navigator);
  }
  function visibility(): void {
    if (document.hidden) {
      for (const element of elements.keys()) update(element);
      if (flushTimer !== undefined) clearTimeout(flushTimer);
      flushTimer = undefined;
      const events = [...pending.values()];
      for (let i = 0; i < events.length; i += 100)
        beaconImpressions(events.slice(i, i + 100), navigator);
    } else {
      refreshVisibility();
      scheduleFlush();
    }
  }
  function online(): void {
    scheduleFlush();
  }
  document.addEventListener('visibilitychange', visibility);
  window.addEventListener('pagehide', unload);
  window.addEventListener('pageshow', resume);
  window.addEventListener('online', online);
  resume();
  const stop = () => {
    unload();
    document.removeEventListener('visibilitychange', visibility);
    window.removeEventListener('pagehide', unload);
    window.removeEventListener('pageshow', resume);
    window.removeEventListener('online', online);
    sessions.delete(document);
  };
  sessions.set(document, stop);
  return stop;
}

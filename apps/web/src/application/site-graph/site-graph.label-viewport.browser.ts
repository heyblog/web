import type { GraphLabelViewport, GraphRect } from './site-graph.labels.ts';

/** Measures the visible part of a graph, including fullscreen panels and floating controls. */
export function observeGraphLabelViewport(
  host: HTMLElement,
  change: (value: GraphLabelViewport) => void,
) {
  const workspace = host.closest('[data-graph-workspace]') ?? host;
  let frame = 0;
  let previous = '';
  const observed = new Set<Element>();
  const measure = () => {
    frame = 0;
    const bounds = host.getBoundingClientRect();
    const visual = window.visualViewport;
    const left = visual?.offsetLeft ?? 0;
    const top = visual?.offsetTop ?? 0;
    const right = left + (visual?.width ?? window.innerWidth);
    const bottom = top + (visual?.height ?? window.innerHeight);
    const x = Math.max(0, left - bounds.left);
    const y = Math.max(0, top - bounds.top);
    const clip: GraphRect = {
      x,
      y,
      width: Math.max(0, Math.min(bounds.width, right - bounds.left) - x),
      height: Math.max(0, Math.min(bounds.height, bottom - bounds.top) - y),
    };
    const elements = [
      ...workspace.querySelectorAll<HTMLElement>('[data-graph-label-obstacle]'),
      ...(workspace.getAttribute('aria-modal') === 'true'
        ? []
        : document.querySelectorAll<HTMLElement>('[data-page-header]')),
    ];
    const obstacles: GraphRect[] = [];
    const current = new Set<Element>([host, workspace, ...elements]);
    for (const element of observed)
      if (!current.has(element)) {
        resize.unobserve(element);
        observed.delete(element);
      }
    for (const element of current)
      if (!observed.has(element)) {
        resize.observe(element);
        observed.add(element);
      }
    for (const element of elements) {
      if (!element.getClientRects().length) continue;
      const rect = element.getBoundingClientRect();
      if (
        rect.right <= bounds.left ||
        rect.left >= bounds.right ||
        rect.bottom <= bounds.top ||
        rect.top >= bounds.bottom
      )
        continue;
      obstacles.push({
        x: rect.left - bounds.left,
        y: rect.top - bounds.top,
        width: rect.width,
        height: rect.height,
      });
    }
    const value = {
      clip,
      obstacles,
      rem: parseFloat(getComputedStyle(document.documentElement).fontSize),
    };
    const key = JSON.stringify(value);
    if (key !== previous) {
      previous = key;
      change(value);
    }
  };
  const schedule = () => {
    if (!frame) frame = requestAnimationFrame(measure);
  };
  const resize = new ResizeObserver(schedule);
  const mutation = new MutationObserver((records) => {
    if (
      records.some(
        (record) =>
          !(record.target instanceof Element) || !record.target.closest('[data-graph-labels]'),
      )
    )
      schedule();
  });
  mutation.observe(workspace, {
    subtree: true,
    childList: true,
    attributes: true,
    attributeFilter: ['class', 'hidden', 'open', 'aria-modal'],
  });
  window.addEventListener('scroll', schedule, { passive: true, capture: true });
  window.addEventListener('resize', schedule);
  window.visualViewport?.addEventListener('resize', schedule);
  window.visualViewport?.addEventListener('scroll', schedule);
  measure();
  return {
    destroy(): void {
      cancelAnimationFrame(frame);
      resize.disconnect();
      mutation.disconnect();
      window.removeEventListener('scroll', schedule, true);
      window.removeEventListener('resize', schedule);
      window.visualViewport?.removeEventListener('resize', schedule);
      window.visualViewport?.removeEventListener('scroll', schedule);
    },
  };
}

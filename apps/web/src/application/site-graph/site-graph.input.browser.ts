import { createGraphGesture } from './site-graph.gesture.ts';
import type { GraphMode } from './site-graph.model.ts';
import type { GraphPoint } from './site-graph.navigation.ts';

export interface GraphInteraction {
  readonly mode: GraphMode;
  readonly fullscreen: boolean;
  readonly touchActive: boolean;
}

interface GraphInputHost extends Pick<
  HTMLElement,
  | 'dispatchEvent'
  | 'clientWidth'
  | 'clientHeight'
  | 'getBoundingClientRect'
  | 'focus'
  | 'setPointerCapture'
  | 'hasPointerCapture'
  | 'releasePointerCapture'
> {
  addEventListener<K extends keyof GlobalEventHandlersEventMap>(
    type: K,
    listener: (event: GlobalEventHandlersEventMap[K]) => void,
    options?: boolean | AddEventListenerOptions,
  ): void;
  removeEventListener<K extends keyof GlobalEventHandlersEventMap>(
    type: K,
    listener: (event: GlobalEventHandlersEventMap[K]) => void,
  ): void;
}

/** One pointer owner for both renderers; browser scrolling remains available in touch browse mode. */
export function graphInput(
  host: GraphInputHost,
  callbacks: {
    readonly interaction: () => GraphInteraction;
    readonly start: (point: GraphPoint) => void;
    readonly pan: (dx: number, dy: number) => void;
    readonly rotate: (dx: number, dy: number) => void;
    readonly zoom: (factor: number, point?: GraphPoint) => void;
    readonly pick: (point: GraphPoint, kind: 'hover' | 'select' | 'focus') => void;
    readonly leave: () => void;
    readonly reset: () => void;
  },
) {
  const gesture = createGraphGesture();
  const pointers = new Map<number, GraphPoint>();
  let right = false;
  let tap: { readonly point: GraphPoint; readonly time: number; readonly type: string } | undefined;
  const center = () => ({ x: host.clientWidth / 2, y: host.clientHeight / 2 });
  const point = (event: PointerEvent | WheelEvent): GraphPoint => {
    const rect = host.getBoundingClientRect();
    return { x: event.clientX - rect.left, y: event.clientY - rect.top };
  };
  const centroid = (): GraphPoint => {
    const values = [...pointers.values()];
    return {
      x: values.reduce((sum, p) => sum + p.x, 0) / values.length,
      y: values.reduce((sum, p) => sum + p.y, 0) / values.length,
    };
  };
  const distance = (): number => {
    const [a, b] = [...pointers.values()];
    return a && b ? Math.hypot(a.x - b.x, a.y - b.y) : 0;
  };
  const down = (event: PointerEvent) => {
    if (event.button !== 0 && event.button !== 2) return;
    const p = point(event);
    gesture.down(event.pointerId, p);
    if (event.pointerType === 'touch' && !callbacks.interaction().touchActive) return;
    pointers.set(event.pointerId, p);
    right = event.button === 2;
    callbacks.start(centroid());
    host.setPointerCapture(event.pointerId);
    host.focus({ preventScroll: true });
  };
  const move = (event: PointerEvent) => {
    const p = point(event);
    gesture.move(event.pointerId, p);
    if (!pointers.has(event.pointerId)) {
      if (event.pointerType === 'mouse') callbacks.pick(p, 'hover');
      return;
    }
    const before = centroid();
    const beforeDistance = distance();
    pointers.set(event.pointerId, p);
    const after = centroid();
    if (pointers.size > 1) {
      callbacks.pan(after.x - before.x, after.y - before.y);
      const nextDistance = distance();
      if (beforeDistance > 0 && nextDistance > 0)
        callbacks.zoom(beforeDistance / nextDistance, after);
    } else if (right || callbacks.interaction().mode === 'pan') {
      callbacks.pan(after.x - before.x, after.y - before.y);
    } else callbacks.rotate(after.x - before.x, after.y - before.y);
  };
  const up = (event: PointerEvent) => {
    const p = point(event);
    const click = gesture.up(event.pointerId, p);
    pointers.delete(event.pointerId);
    if (host.hasPointerCapture(event.pointerId)) host.releasePointerCapture(event.pointerId);
    if (pointers.size) callbacks.start(centroid());
    if (!click || event.button !== 0) {
      tap = undefined;
      return;
    }
    const now = performance.now();
    const double =
      (event.pointerType !== 'touch' || callbacks.interaction().touchActive) &&
      tap?.type === event.pointerType &&
      now - tap.time < 320 &&
      Math.hypot(tap.point.x - p.x, tap.point.y - p.y) < 24;
    callbacks.pick(p, double ? 'focus' : 'select');
    tap = double ? undefined : { point: p, time: now, type: event.pointerType };
  };
  const cancel = () => {
    const ids = [...pointers.keys()];
    pointers.clear();
    gesture.cancel();
    tap = undefined;
    for (const id of ids) if (host.hasPointerCapture(id)) host.releasePointerCapture(id);
  };
  const lost = (event: PointerEvent) => {
    if (pointers.has(event.pointerId)) cancel();
  };
  const wheel = (event: WheelEvent) => {
    event.preventDefault();
    const unit = event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? host.clientHeight : 1;
    callbacks.zoom(
      Math.exp(Math.max(-120, Math.min(120, event.deltaY * unit)) * 0.002),
      point(event),
    );
  };
  const key = (event: KeyboardEvent) => {
    if (event.target !== host) return;
    if (event.key === '+' || event.key === '=') callbacks.zoom(0.8);
    else if (event.key === '-') callbacks.zoom(1.25);
    else if (event.key === 'Home') callbacks.reset();
    else if (['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) {
      callbacks.start(center());
      callbacks.pan(
        event.key === 'ArrowLeft' ? 40 : event.key === 'ArrowRight' ? -40 : 0,
        event.key === 'ArrowUp' ? 40 : event.key === 'ArrowDown' ? -40 : 0,
      );
    } else return;
    event.preventDefault();
  };
  const context = (event: Event) => event.preventDefault();
  host.addEventListener('pointerdown', down);
  host.addEventListener('pointermove', move);
  host.addEventListener('pointerup', up);
  host.addEventListener('pointercancel', cancel);
  host.addEventListener('lostpointercapture', lost);
  host.addEventListener('pointerleave', callbacks.leave);
  host.addEventListener('wheel', wheel, { passive: false });
  host.addEventListener('keydown', key);
  host.addEventListener('contextmenu', context);
  return {
    destroy(): void {
      cancel();
      host.removeEventListener('pointerdown', down);
      host.removeEventListener('pointermove', move);
      host.removeEventListener('pointerup', up);
      host.removeEventListener('pointercancel', cancel);
      host.removeEventListener('lostpointercapture', lost);
      host.removeEventListener('pointerleave', callbacks.leave);
      host.removeEventListener('wheel', wheel);
      host.removeEventListener('keydown', key);
      host.removeEventListener('contextmenu', context);
    },
  };
}

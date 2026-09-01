import { type BlogCardSourceRect, resolveAnchoredDialogLayout } from './blog-card-layout.shared';

const focusableSelector =
  'a[href],button,input,select,textarea,[contenteditable]:not([contenteditable="false"]),[tabindex]';
const MAX_MOTION_DISTANCE = 16;
const VIEWPORT_INSET = 16;

export function readMotionDuration(
  element: HTMLElement,
  property: string,
  fallback: number,
): number {
  const rawValue = getComputedStyle(element).getPropertyValue(property).trim();
  if (rawValue.endsWith('ms')) return Number.parseFloat(rawValue) || fallback;
  if (rawValue.endsWith('s')) return (Number.parseFloat(rawValue) || fallback / 1000) * 1000;
  return fallback;
}

export function captureSourceRect(sourceElement: HTMLElement | null): BlogCardSourceRect | null {
  if (!sourceElement) return null;
  const bounds = sourceElement.getBoundingClientRect();
  return {
    left: bounds.left,
    top: bounds.top,
    width: bounds.width,
    height: bounds.height,
  };
}

export function applyDialogLayout(dialog: HTMLDialogElement, source: BlogCardSourceRect): void {
  const viewport = { width: window.innerWidth, height: window.innerHeight };
  const provisionalWidth = Math.min(source.width, Math.max(0, viewport.width - VIEWPORT_INSET * 2));
  dialog.style.width = `${provisionalWidth}px`;
  dialog.style.maxHeight = `${Math.max(0, viewport.height - VIEWPORT_INSET * 2)}px`;
  const layout = resolveAnchoredDialogLayout({
    source,
    dialogHeight: Math.max(dialog.getBoundingClientRect().height, dialog.scrollHeight),
    viewport,
  });
  dialog.style.left = `${layout.left}px`;
  dialog.style.top = `${layout.top}px`;
  dialog.style.width = `${layout.width}px`;
  dialog.style.maxHeight = `${layout.maxHeight}px`;
}

export function sourceTransform(dialog: HTMLDialogElement, source: BlogCardSourceRect): string {
  const target = dialog.getBoundingClientRect();
  const translateX = Math.min(
    MAX_MOTION_DISTANCE,
    Math.max(-MAX_MOTION_DISTANCE, source.left - target.left),
  );
  const translateY = Math.min(
    MAX_MOTION_DISTANCE,
    Math.max(-MAX_MOTION_DISTANCE, source.top - target.top),
  );
  const scaleX = Math.max(0.96, Math.min(1, source.width / target.width));
  const scaleY = Math.max(0.96, Math.min(1, source.height / target.height));
  return `translate(${translateX}px, ${translateY}px) scale(${scaleX}, ${scaleY})`;
}

export function findFocusableElements(dialog: HTMLDialogElement): readonly HTMLElement[] {
  return [...dialog.querySelectorAll<HTMLElement>(focusableSelector)].filter(
    (element) =>
      element.tabIndex >= 0 &&
      !element.matches(':disabled') &&
      element.getClientRects().length > 0 &&
      getComputedStyle(element).visibility !== 'hidden',
  );
}

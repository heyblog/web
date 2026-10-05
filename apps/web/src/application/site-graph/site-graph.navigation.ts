export interface GraphPoint {
  readonly x: number;
  readonly y: number;
}

export const graphZoom = { min: 0.25, max: 1024 } as const;
export function graphZoomLevel(value: number): number {
  // Repeated wheel/pinch ratios can land a few ulps short of a boundary.
  if (value >= graphZoom.max - 1e-9) return graphZoom.max;
  if (value <= graphZoom.min + 1e-9) return graphZoom.min;
  return value;
}

export const graphNodeSize = (base: number, zoom: number): number =>
  Math.min(base === 12 ? 28 : base === 6 ? 22 : 18, base * Math.sqrt(Math.max(1, zoom)));

export const graphLineOpacity = (zoom: number): number =>
  0.06 + 0.08 * Math.min(1, Math.max(0, Math.log2(zoom) / 6));

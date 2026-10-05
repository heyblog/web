import type { GraphPoint } from './site-graph.navigation.ts';

export interface GraphRect {
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}

export interface GraphLabelViewport {
  readonly clip: GraphRect;
  readonly obstacles: readonly GraphRect[];
  readonly rem: number;
}

export interface GraphLabelNode extends GraphPoint {
  readonly id: string;
  readonly radius: number;
  /** Hover, selection, path, center, then ordinary nodes. */
  readonly priority: number;
  readonly persistent: boolean;
}

export interface GraphLabel extends GraphRect {
  readonly id: string;
  readonly nodeX: number;
  readonly nodeY: number;
}

export function graphRectsOverlap(a: GraphRect, b: GraphRect, gap = 0): boolean {
  return (
    a.x < b.x + b.width + gap &&
    a.x + a.width + gap > b.x &&
    a.y < b.y + b.height + gap &&
    a.y + a.height + gap > b.y
  );
}

function contains(rect: GraphRect, point: GraphPoint): boolean {
  return (
    point.x >= rect.x &&
    point.x <= rect.x + rect.width &&
    point.y >= rect.y &&
    point.y <= rect.y + rect.height
  );
}

export function graphNodeOnScreen(point: GraphPoint, viewport: GraphLabelViewport): boolean {
  return (
    Number.isFinite(point.x) &&
    Number.isFinite(point.y) &&
    viewport.clip.width > 0 &&
    viewport.clip.height > 0 &&
    contains(viewport.clip, point) &&
    !viewport.obstacles.some((rect) => contains(rect, point))
  );
}

/** Each graph owns its hysteresis and stable placement order. No browser or Three dependency. */
export function createGraphLabels() {
  let automatic = false;
  let previous = new Set<string>();
  return {
    update(nodes: readonly GraphLabelNode[], viewport: GraphLabelViewport) {
      const shown = nodes.filter((node) => graphNodeOnScreen(node, viewport));
      const wide = viewport.clip.width >= 48 * viewport.rem;
      const enter = wide ? 20 : 8;
      const exit = wide ? 24 : 10;
      automatic = shown.length > 0 && shown.length <= (automatic ? exit : enter);
      const candidates = shown
        .filter((node) => automatic || node.persistent)
        .sort(
          (a, b) =>
            b.priority - a.priority ||
            Number(previous.has(b.id)) - Number(previous.has(a.id)) ||
            a.id.localeCompare(b.id),
        );
      const labels: GraphLabel[] = [];
      const width = Math.min((wide ? 12 : 10) * viewport.rem, viewport.clip.width - 16);
      const height = 3 * viewport.rem;
      const gap = 4;
      const inBounds = (rect: GraphRect) =>
        rect.x >= viewport.clip.x + gap &&
        rect.y >= viewport.clip.y + gap &&
        rect.x + rect.width <= viewport.clip.x + viewport.clip.width - gap &&
        rect.y + rect.height <= viewport.clip.y + viewport.clip.height - gap;
      for (const node of candidates) {
        if (width <= 0 || height + gap * 2 > viewport.clip.height) break;
        const x = Math.min(
          viewport.clip.x + viewport.clip.width - width - gap,
          Math.max(viewport.clip.x + gap, node.x - width / 2),
        );
        const y = node.y - node.radius - 8 - height;
        const options = [
          { x, y, width, height },
          { x: x - width / 2, y, width, height },
          { x: x + width / 2, y, width, height },
          { x, y: y - height - gap, width, height },
        ];
        if (node.persistent) options.push({ x, y: node.y + node.radius + 8, width, height });
        const fits = (rect: GraphRect, avoidNodes: boolean) =>
          inBounds(rect) &&
          !viewport.obstacles.some((obstacle) => graphRectsOverlap(rect, obstacle, gap)) &&
          !labels.some((label) => graphRectsOverlap(rect, label, gap)) &&
          (!avoidNodes ||
            !shown.some((other) =>
              graphRectsOverlap(
                rect,
                {
                  x: other.x - other.radius,
                  y: other.y - other.radius,
                  width: other.radius * 2,
                  height: other.radius * 2,
                },
                gap,
              ),
            ));
        // Persistent details remain readable even when the surrounding graph is dense.
        const rect =
          options.find((option) => fits(option, true)) ??
          (node.persistent ? options.find((option) => fits(option, false)) : undefined);
        if (rect) labels.push({ ...rect, id: node.id, nodeX: node.x, nodeY: node.y });
      }
      previous = new Set(labels.map((label) => label.id));
      return { labels, count: shown.length, automatic };
    },
  };
}

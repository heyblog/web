import { Color, Vector3 } from 'three';

import type { SiteGraph } from '../../api/sites/site-graph.types.ts';

export function graphPalette(host: HTMLElement) {
  const style = getComputedStyle(host);
  const color = (name: string) => new Color(style.getPropertyValue(`--sem-${name}`).trim());
  return {
    background: color('canvas'),
    registered: color('tint-fg'),
    external: color('warning-fg'),
    line: color('fg-muted'),
    highlight: color('info-fg'),
    reciprocal: color('success-fg'),
  };
}

export function pathEdges(path: readonly string[]): ReadonlySet<string> {
  const pairs = new Set<string>();
  for (let index = 1; index < path.length; index++) {
    pairs.add(JSON.stringify([path[index - 1], path[index]]));
    pairs.add(JSON.stringify([path[index], path[index - 1]]));
  }
  return pairs;
}

export function graphArrows(
  graph: SiteGraph,
  positions: Float32Array,
  focus: string,
  steps: ReadonlySet<string>,
): Float32Array {
  const indices = new Map(graph.nodes.map((node, index) => [node.id, index]));
  const vertices: number[] = [];
  for (const edge of graph.edges) {
    if (
      edge.source !== focus &&
      edge.target !== focus &&
      !steps.has(JSON.stringify([edge.source, edge.target]))
    )
      continue;
    const source = indices.get(edge.source);
    const target = indices.get(edge.target);
    if (source === undefined || target === undefined) continue;
    const a = new Vector3().fromArray(positions, source * 3);
    const b = new Vector3().fromArray(positions, target * 3);
    const direction = b.clone().sub(a).normalize();
    const wing = direction
      .clone()
      .cross(Math.abs(direction.y) > 0.9 ? new Vector3(1, 0, 0) : new Vector3(0, 1, 0))
      .normalize()
      .multiplyScalar(1.5);
    const tip = a.clone().lerp(b, 0.7);
    const base = tip.clone().addScaledVector(direction, -4);
    vertices.push(
      ...tip.toArray(),
      ...base.clone().add(wing).toArray(),
      ...tip.toArray(),
      ...base.sub(wing).toArray(),
    );
  }
  return new Float32Array(vertices);
}

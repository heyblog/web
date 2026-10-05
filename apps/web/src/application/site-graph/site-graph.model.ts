import type { GraphNode, SiteGraph } from '../../api/sites/site-graph.types.ts';

export type GraphRelation = 'all' | 'incoming' | 'outgoing' | 'reciprocal';
export type GraphMode = 'rotate' | 'pan';
export interface GraphDisplay {
  readonly isolated: boolean;
  readonly scope: 'all' | 'neighbors';
}

export function indexGraph(graph: SiteGraph) {
  const nodes = new Map(graph.nodes.map((node, index) => [node.id, { node, index }]));
  const adjacent = new Map<string, Set<string>>();
  const incoming = new Map<string, Set<string>>();
  const outgoing = new Map<string, Set<string>>();
  for (const node of graph.nodes) {
    adjacent.set(node.id, new Set());
    incoming.set(node.id, new Set());
    outgoing.set(node.id, new Set());
  }
  for (const edge of graph.edges) {
    adjacent.get(edge.source)?.add(edge.target);
    adjacent.get(edge.target)?.add(edge.source);
    outgoing.get(edge.source)?.add(edge.target);
    incoming.get(edge.target)?.add(edge.source);
  }
  const connected = new Set(
    graph.nodes.filter((node) => adjacent.get(node.id)?.size).map((node) => node.id),
  );
  function relations(id: string) {
    const ins = incoming.get(id) ?? new Set<string>();
    const outs = outgoing.get(id) ?? new Set<string>();
    const project = (ids: Iterable<string>): readonly GraphNode[] =>
      Array.from(ids).flatMap((key) => {
        const entry = nodes.get(key);
        return entry ? [entry.node] : [];
      });
    return {
      all: project(adjacent.get(id) ?? []),
      incoming: project(ins),
      outgoing: project(outs),
      reciprocal: project(Array.from(outs).filter((key) => ins.has(key))),
    };
  }
  return { nodes, adjacent, connected, relations };
}
export type GraphIndex = ReturnType<typeof indexGraph>;

export function visibleGraphNodes(
  index: GraphIndex,
  options: GraphDisplay & {
    readonly focus: string;
    readonly path: readonly string[];
    readonly center?: string;
    readonly relation?: GraphRelation;
  },
): ReadonlySet<string> {
  if (options.center) {
    if (!index.nodes.has(options.center)) return new Set();
    return new Set([
      options.center,
      ...index.relations(options.center)[options.relation ?? 'all'].map((node) => node.id),
    ]);
  }
  if (options.scope === 'neighbors' && options.focus) {
    return new Set([options.focus, ...(index.adjacent.get(options.focus) ?? []), ...options.path]);
  }
  return options.isolated ? new Set(index.nodes.keys()) : index.connected;
}

/** Stable, disjoint sectors preserve a site's relationship directions while filtering. */
export function localGraphLayout(graph: SiteGraph): Float32Array {
  const index = indexGraph(graph);
  const center = graph.centerId ?? '';
  const relations = index.relations(center);
  const mutual = new Set(relations.reciprocal.map((node) => node.id));
  const groups = [
    relations.incoming.filter((node) => !mutual.has(node.id)),
    relations.reciprocal,
    relations.outgoing.filter((node) => !mutual.has(node.id)),
  ];
  const occupied = groups.filter((group) => group.length > 0).length;
  const positions = new Float32Array(graph.nodes.length * 3);
  groups.forEach((group, sector) => {
    const sorted = [...group].sort((a, b) => a.id.localeCompare(b.id));
    const span = occupied === 1 ? Math.PI * 2 : (Math.PI * 2) / 3 - 0.2;
    const start = occupied === 1 ? -Math.PI / 2 : Math.PI / 2 + sector * ((Math.PI * 2) / 3);
    let offset = 0;
    for (let ring = 0; offset < sorted.length; ring++) {
      const radius = 100 + ring * 64;
      const capacity = Math.max(3, Math.floor((radius * span) / 56));
      const count = Math.min(capacity, sorted.length - offset);
      for (let item = 0; item < count; item++) {
        const node = sorted[offset + item];
        const entry = node && index.nodes.get(node.id);
        if (!entry) continue;
        const angle = start + ((item + 0.5) / count) * span;
        positions[entry.index * 3] = Math.cos(angle) * radius;
        positions[entry.index * 3 + 1] = Math.sin(angle) * radius;
      }
      offset += count;
    }
  });
  return positions;
}

export function clampZoom(value: number, minimum: number, maximum: number): number {
  return Math.min(maximum, Math.max(minimum, value));
}

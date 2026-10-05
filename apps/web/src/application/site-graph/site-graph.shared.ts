import type { GraphNode, SiteGraph } from '../../api/sites/site-graph.types.ts';

export interface GraphSelection {
  readonly focus: string;
  readonly from: string;
  readonly to: string;
  readonly direction: 'directed' | 'undirected';
}

export function readGraphSelection(parameters: URLSearchParams): GraphSelection {
  return {
    focus: parameters.get('focus') ?? '',
    from: parameters.get('from') ?? '',
    to: parameters.get('to') ?? '',
    direction: parameters.get('direction') === 'undirected' ? 'undirected' : 'directed',
  };
}

export function graphHref(selection: Partial<GraphSelection>): string {
  const parameters = new URLSearchParams();
  for (const key of ['focus', 'from', 'to', 'direction'] as const) {
    const value = selection[key];
    if (value) parameters.set(key, value);
  }
  return `/graph${parameters.size ? `?${parameters}` : ''}`;
}

export function readGraphDisplay(parameters: URLSearchParams) {
  return {
    isolated: parameters.get('isolated') === '1',
    scope: parameters.get('scope') === 'neighbors' ? ('neighbors' as const) : ('all' as const),
  };
}

export function graphViewHref(
  selection: Partial<GraphSelection>,
  display: { readonly isolated: boolean; readonly scope: 'all' | 'neighbors' },
): string {
  const parameters = new URLSearchParams(graphHref(selection).split('?')[1]);
  if (display.isolated) parameters.set('isolated', '1');
  if (display.scope === 'neighbors') parameters.set('scope', 'neighbors');
  return `/graph${parameters.size ? `?${parameters}` : ''}`;
}

export function nodeDetailHref(node: GraphNode): string | null {
  if (!node.shortId) return null;
  return `${node.customId ? `/s/${encodeURIComponent(node.customId)}` : `/site/${encodeURIComponent(node.shortId)}`}#links`;
}

export function graphRelations(graph: SiteGraph, id: string) {
  const nodes = new Map(graph.nodes.map((node) => [node.id, node]));
  const incoming: GraphNode[] = [];
  const outgoing: GraphNode[] = [];
  const reciprocal: GraphNode[] = [];
  for (const edge of graph.edges) {
    if (edge.target === id) {
      const node = nodes.get(edge.source);
      if (node) incoming.push(node);
    }
    if (edge.source === id) {
      const node = nodes.get(edge.target);
      if (node) {
        outgoing.push(node);
        if (edge.reciprocal) reciprocal.push(node);
      }
    }
  }
  return { incoming, outgoing, reciprocal };
}

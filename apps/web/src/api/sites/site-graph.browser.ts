import type { GraphEdge, GraphNode, SiteGraph } from './site-graph.types.ts';

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function nullableString(value: unknown): value is string | null {
  return value === null || typeof value === 'string';
}

function node(value: unknown): value is GraphNode {
  if (!record(value)) return false;
  if (!['id', 'name', 'host', 'homepageUrl'].every((key) => typeof value[key] === 'string'))
    return false;
  if (typeof value.homepageUrl !== 'string') return false;
  try {
    const url = new URL(value.homepageUrl);
    if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password) return false;
  } catch {
    return false;
  }
  return nullableString(value.shortId) && nullableString(value.customId);
}

function edge(value: unknown): value is GraphEdge {
  return (
    record(value) &&
    typeof value.source === 'string' &&
    typeof value.target === 'string' &&
    typeof value.reciprocal === 'boolean'
  );
}

export function parseSiteGraph(value: unknown): SiteGraph {
  if (
    !record(value) ||
    !Array.isArray(value.nodes) ||
    !Array.isArray(value.edges) ||
    !value.nodes.every(node) ||
    !value.edges.every(edge) ||
    !nullableString(value.centerId) ||
    !record(value.stats)
  ) {
    throw new TypeError('Invalid graph response');
  }
  const { nodes, edges, stats, centerId } = value;
  const ids = new Set(nodes.map((item) => item.id));
  const pairs = new Set<string>();
  if (
    ids.size !== nodes.length ||
    edges.some(
      (item) =>
        !ids.has(item.source) ||
        !ids.has(item.target) ||
        item.source === item.target ||
        pairs.size === pairs.add(JSON.stringify([item.source, item.target])).size,
    ) ||
    (centerId !== null && nodes.length > 0 && !ids.has(centerId)) ||
    typeof stats.nodes !== 'number' ||
    typeof stats.edges !== 'number' ||
    typeof stats.reciprocalPairs !== 'number' ||
    !Number.isSafeInteger(stats.reciprocalPairs) ||
    stats.reciprocalPairs < 0 ||
    stats.nodes !== nodes.length ||
    stats.edges !== edges.length
  ) {
    throw new TypeError('Inconsistent graph response');
  }
  return {
    nodes,
    edges,
    centerId,
    stats: { nodes: stats.nodes, edges: stats.edges, reciprocalPairs: stats.reciprocalPairs },
  };
}

export async function requestSiteGraph(
  identifier: string | undefined,
  signal: AbortSignal,
): Promise<SiteGraph> {
  const path = identifier ? `/api/site-graph/${encodeURIComponent(identifier)}` : '/api/site-graph';
  const response = await fetch(path, {
    signal: AbortSignal.any([signal, AbortSignal.timeout(30_000)]),
    cache: 'no-store',
    headers: { Accept: 'application/json' },
  });
  if (!response.ok) throw new TypeError('Graph request failed');
  return parseSiteGraph(await response.json());
}

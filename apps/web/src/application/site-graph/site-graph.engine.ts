import { DirectedGraph, UndirectedGraph } from 'graphology';
import { bidirectional } from 'graphology-shortest-path/unweighted.js';

import type { SiteGraph } from '../../api/sites/site-graph.types.ts';

export function createPathFinder(data: SiteGraph) {
  const directed = new DirectedGraph();
  const undirected = new UndirectedGraph();
  for (const node of data.nodes) {
    directed.addNode(node.id);
    undirected.addNode(node.id);
  }
  for (const edge of data.edges) {
    directed.mergeEdge(edge.source, edge.target);
    undirected.mergeEdge(edge.source, edge.target);
  }
  return (from: string, to: string, direction: 'directed' | 'undirected'): string[] | null => {
    if (!directed.hasNode(from) || !directed.hasNode(to)) return null;
    return bidirectional(direction === 'directed' ? directed : undirected, from, to);
  };
}

function compareText(left: string, right: string): number {
  return left < right ? -1 : left > right ? 1 : 0;
}

function normalizeSearch(value: string): string {
  return value.normalize('NFKC').trim().toLowerCase();
}

export function createGraphSearch(data: SiteGraph) {
  const degree = new Map<string, number>();
  for (const edge of data.edges) {
    degree.set(edge.source, (degree.get(edge.source) ?? 0) + 1);
    degree.set(edge.target, (degree.get(edge.target) ?? 0) + 1);
  }
  const entries = data.nodes
    .map((node) => ({
      id: node.id,
      name: normalizeSearch(node.name),
      host: normalizeSearch(node.host),
      degree: degree.get(node.id) ?? 0,
    }))
    .sort(
      (left, right) =>
        right.degree - left.degree ||
        compareText(left.host, right.host) ||
        compareText(left.id, right.id),
    );
  return (query: string, limit: number): { ids: readonly string[]; total: number } => {
    const normalized = normalizeSearch(query);
    const size = Number.isFinite(limit) ? Math.max(0, Math.trunc(limit)) : entries.length;
    if (!normalized)
      return { ids: entries.slice(0, size).map((entry) => entry.id), total: entries.length };
    const ranked: string[][] = [[], [], []];
    let total = 0;
    for (const entry of entries) {
      const rank =
        entry.host === normalized || entry.name === normalized
          ? 0
          : entry.host.startsWith(normalized) || entry.name.startsWith(normalized)
            ? 1
            : entry.host.includes(normalized) || entry.name.includes(normalized)
              ? 2
              : -1;
      if (rank < 0) continue;
      total++;
      if ((ranked[rank]?.length ?? 0) < size) ranked[rank]?.push(entry.id);
    }
    return { ids: ranked.flat().slice(0, size), total };
  };
}

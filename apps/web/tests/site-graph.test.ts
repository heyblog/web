import assert from 'node:assert/strict';
import test from 'node:test';

import { parseSiteGraph } from '../src/api/sites/site-graph.browser.ts';
import type { GraphNode, SiteGraph } from '../src/api/sites/site-graph.types.ts';
import { createPathFinder } from '../src/application/site-graph/site-graph.engine.ts';
import {
  graphHref,
  graphRelations,
  nodeDetailHref,
  readGraphSelection,
} from '../src/application/site-graph/site-graph.shared.ts';

const node = (id: string, shortId: string | null = id): GraphNode => ({
  id,
  name: id,
  host: `${id}.example`,
  homepageUrl: `https://${id}.example/`,
  shortId,
  customId: null,
});
const graph: SiteGraph = {
  nodes: [node('a'), node('b'), node('c', null), node('isolated')],
  edges: [
    { source: 'a', target: 'b', reciprocal: true },
    { source: 'b', target: 'a', reciprocal: true },
    { source: 'b', target: 'c', reciprocal: false },
  ],
  stats: { nodes: 4, edges: 3, reciprocalPairs: 1 },
  centerId: null,
};

test('graph boundary accepts isolated and external nodes and rejects malformed topology and URLs', () => {
  assert.deepEqual(parseSiteGraph(graph), graph);
  assert.deepEqual(
    parseSiteGraph({
      nodes: [],
      edges: [],
      centerId: 'site:hidden',
      stats: { nodes: 0, edges: 0, reciprocalPairs: 0 },
    }),
    {
      nodes: [],
      edges: [],
      centerId: 'site:hidden',
      stats: { nodes: 0, edges: 0, reciprocalPairs: 0 },
    },
  );
  for (const invalid of [
    null,
    {},
    { ...graph, stats: { ...graph.stats, nodes: 3 } },
    { ...graph, centerId: 'missing' },
    { ...graph, nodes: [...graph.nodes, node('a')] },
    { ...graph, nodes: [{ ...node('a'), homepageUrl: 'data:text/html,test' }] },
    { ...graph, edges: [{ source: 'a', target: 'missing', reciprocal: false }] },
    { ...graph, edges: [{ source: 'a', target: 'a', reciprocal: false }] },
    { ...graph, edges: [graph.edges[0], graph.edges[0]] },
  ]) {
    assert.throws(() => parseSiteGraph(invalid), TypeError);
  }
});

test('path finder respects directions, reciprocity, missing nodes, isolation and same-node paths', () => {
  const path = createPathFinder(graph);
  assert.deepEqual(path('a', 'c', 'directed'), ['a', 'b', 'c']);
  assert.equal(path('c', 'a', 'directed'), null);
  assert.deepEqual(path('c', 'a', 'undirected'), ['c', 'b', 'a']);
  assert.deepEqual(path('b', 'a', 'directed'), ['b', 'a']);
  assert.deepEqual(path('a', 'a', 'directed'), ['a']);
  assert.equal(path('isolated', 'a', 'undirected'), null);
  assert.equal(path('missing', 'a', 'directed'), null);
});

test('relation lists preserve incoming, outgoing and reciprocal semantics', () => {
  const relations = graphRelations(graph, 'b');
  assert.deepEqual(
    relations.incoming.map((item) => item.id),
    ['a'],
  );
  assert.deepEqual(
    relations.outgoing.map((item) => item.id),
    ['a', 'c'],
  );
  assert.deepEqual(
    relations.reciprocal.map((item) => item.id),
    ['a'],
  );
  assert.deepEqual(graphRelations(graph, 'isolated'), {
    incoming: [],
    outgoing: [],
    reciprocal: [],
  });
});

test('graph share links round-trip selection and detail routes escape identifiers', () => {
  const selection = {
    focus: 'external:example.com',
    from: 'a',
    to: 'b',
    direction: 'undirected' as const,
  };
  const url = new URL(graphHref(selection), 'https://heyblog.example');
  assert.deepEqual(readGraphSelection(url.searchParams), selection);
  assert.equal(
    readGraphSelection(new URLSearchParams('direction=unexpected')).direction,
    'directed',
  );
  assert.equal(nodeDetailHref(node('external', null)), null);
  assert.equal(nodeDetailHref(node('a')), '/site/a#links');
  assert.equal(nodeDetailHref({ ...node('a'), customId: 'my/blog' }), '/s/my%2Fblog#links');
});

test('large complete graph keeps every node and computes paths without sampling', () => {
  const nodes = Array.from({ length: 50_000 }, (_, index) => node(`n${index}`));
  const edges = nodes
    .slice(1)
    .map((item, index) => ({ source: nodes[index]?.id ?? '', target: item.id, reciprocal: false }));
  const large: SiteGraph = {
    nodes,
    edges,
    stats: { nodes: nodes.length, edges: edges.length, reciprocalPairs: 0 },
    centerId: null,
  };
  const path = createPathFinder(large);
  assert.equal(path('n0', 'n49999', 'directed')?.length, 50_000);
  assert.equal(path('n49999', 'n0', 'directed'), null);
  assert.equal(path('n49999', 'n0', 'undirected')?.length, 50_000);
});

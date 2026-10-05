import assert from 'node:assert/strict';
import test from 'node:test';

import type { SiteGraph } from '../src/api/sites/site-graph.types.ts';
import { createGraphGesture } from '../src/application/site-graph/site-graph.gesture.ts';
import {
  clampZoom,
  indexGraph,
  localGraphLayout,
  visibleGraphNodes,
} from '../src/application/site-graph/site-graph.model.ts';
import {
  graphViewHref,
  readGraphDisplay,
  readGraphSelection,
} from '../src/application/site-graph/site-graph.shared.ts';

const graph: SiteGraph = {
  nodes: ['center', 'in', 'out', 'mutual', 'isolated'].map((id) => ({
    id,
    name: id,
    host: `${id}.test`,
    homepageUrl: `https://${id}.test`,
    shortId: id,
    customId: null,
  })),
  edges: [
    { source: 'in', target: 'center', reciprocal: false },
    { source: 'center', target: 'out', reciprocal: false },
    { source: 'center', target: 'mutual', reciprocal: true },
    { source: 'mutual', target: 'center', reciprocal: true },
  ],
  centerId: 'center',
  stats: { nodes: 5, edges: 4, reciprocalPairs: 1 },
};

test('default display omits only isolated nodes without changing the source topology', () => {
  // Given a full public topology, including a real isolated blog.
  const index = indexGraph(graph);
  // When default and expanded projections are requested.
  const defaults = visibleGraphNodes(index, { isolated: false, scope: 'all', focus: '', path: [] });
  const expanded = visibleGraphNodes(index, { isolated: true, scope: 'all', focus: '', path: [] });
  // Then only the display changes, preserving the complete searchable index.
  assert.deepEqual([...defaults], ['center', 'in', 'out', 'mutual']);
  assert.equal(expanded.size, 5);
  assert.equal(index.nodes.size, 5);
  assert.equal(graph.stats.nodes, 5);
});

test('related view retains all path nodes and the selected neighborhood', () => {
  const index = indexGraph(graph);
  const visible = visibleGraphNodes(index, {
    isolated: false,
    scope: 'neighbors',
    focus: 'in',
    path: ['in', 'center', 'out'],
  });
  assert.deepEqual([...visible].sort(), ['center', 'in', 'out']);
});

test('local filters retain the center and include reciprocal neighbors in both directions', () => {
  const index = indexGraph(graph);
  const display = {
    isolated: false,
    scope: 'all',
    focus: 'center',
    path: [],
    center: 'center',
  } as const;
  assert.deepEqual([...visibleGraphNodes(index, { ...display, relation: 'incoming' })].sort(), [
    'center',
    'in',
    'mutual',
  ]);
  assert.deepEqual([...visibleGraphNodes(index, { ...display, relation: 'outgoing' })].sort(), [
    'center',
    'mutual',
    'out',
  ]);
  assert.deepEqual(
    [...visibleGraphNodes(index, { ...display, relation: 'reciprocal' })],
    ['center', 'mutual'],
  );
});

test('local layout is deterministic, centered and separates every direct neighbor', () => {
  const positions = localGraphLayout(graph);
  assert.deepEqual(positions, localGraphLayout(graph));
  assert.deepEqual(Array.from(positions.slice(0, 3)), [0, 0, 0]);
  const points = Array.from({ length: 4 }, (_, index) =>
    Array.from(positions.slice(index * 3, index * 3 + 3)).join(','),
  );
  assert.equal(new Set(points).size, 4);
});

test('an unavailable center in an empty site response does not create a phantom node', () => {
  const empty = indexGraph({
    nodes: [],
    edges: [],
    centerId: 'site:hidden',
    stats: { nodes: 0, edges: 0, reciprocalPairs: 0 },
  });
  assert.equal(
    visibleGraphNodes(empty, {
      isolated: false,
      scope: 'all',
      focus: '',
      path: [],
      center: 'site:hidden',
    }).size,
    0,
  );
});

test('a drag returning to its origin is never interpreted as a click', () => {
  const gesture = createGraphGesture();
  gesture.down(1, { x: 20, y: 20 });
  gesture.move(1, { x: 80, y: 20 });
  assert.equal(gesture.up(1, { x: 20, y: 20 }), false);
});

test('a two-finger gesture suppresses both releases but a subsequent tap works', () => {
  const gesture = createGraphGesture();
  gesture.down(1, { x: 20, y: 20 });
  gesture.down(2, { x: 40, y: 20 });
  assert.equal(gesture.up(1, { x: 20, y: 20 }), false);
  assert.equal(gesture.up(2, { x: 40, y: 20 }), false);
  gesture.down(3, { x: 20, y: 20 });
  assert.equal(gesture.up(3, { x: 21, y: 20 }), true);
});

test('cancelled pointers cannot select a node on release', () => {
  const gesture = createGraphGesture();
  gesture.down(1, { x: 20, y: 20 });
  gesture.cancel();
  assert.equal(gesture.up(1, { x: 20, y: 20 }), false);
});

test('zoom clamps avoid zero scales and excessive camera travel', () => {
  assert.equal(clampZoom(0, 0.04, 8), 0.04);
  assert.equal(clampZoom(100, 0.04, 8), 8);
  assert.equal(clampZoom(2, 0.04, 8), 2);
});

test('display links round-trip with existing selection links and safe defaults', () => {
  const display = { isolated: true, scope: 'neighbors' } as const;
  const query = new URL(graphViewHref({ focus: 'isolated' }, display), 'https://heyblog.test')
    .searchParams;
  assert.deepEqual(readGraphDisplay(query), display);
  assert.equal(readGraphSelection(query).focus, 'isolated');
  assert.deepEqual(readGraphDisplay(new URLSearchParams('scope=invalid')), {
    isolated: false,
    scope: 'all',
  });
});

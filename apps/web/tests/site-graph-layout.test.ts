import assert from 'node:assert/strict';
import test from 'node:test';

import type { GraphEdge, GraphNode, SiteGraph } from '../src/api/sites/site-graph.types.ts';
import { createGraphSearch } from '../src/application/site-graph/site-graph.engine.ts';
import { createInitialLayout } from '../src/application/site-graph/site-graph.layout.ts';

import { scheduledLayout, workerHarness } from './site-graph-layout.fixture.ts';

const node = (id: string, name = id, host = `${id}.example`): GraphNode => ({
  id,
  name,
  host,
  homepageUrl: `https://${host}/`,
  shortId: id,
  customId: null,
});
function graph(
  nodes: readonly GraphNode[],
  pairs: readonly (readonly [string, string])[],
): SiteGraph {
  const edges: GraphEdge[] = pairs.map(([source, target]) => ({
    source,
    target,
    reciprocal: false,
  }));
  return {
    nodes,
    edges,
    centerId: null,
    stats: { nodes: nodes.length, edges: edges.length, reciprocalPairs: 0 },
  };
}
const connectedGraph = graph(
  ['a', 'b', 'c', 'd', 'e'].map((id) => node(id)),
  [
    ['a', 'b'],
    ['b', 'c'],
    ['d', 'e'],
  ],
);
function coordinates(data: SiteGraph, positions: Float32Array) {
  return new Map(
    data.nodes.map((item, index) => [item.id, [...positions.slice(index * 3, index * 3 + 3)]]),
  );
}

function axisSpread(positions: Float32Array): number[] {
  return [0, 1, 2].map((axis) => {
    const values = Array.from(
      { length: positions.length / 3 },
      (_, i) => positions[i * 3 + axis] ?? 0,
    );
    const mean = values.reduce((sum, value) => sum + value, 0) / values.length;
    return Math.sqrt(values.reduce((sum, value) => sum + (value - mean) ** 2, 0) / values.length);
  });
}

test('a large connected component occupies comparable depth, width and height before and after relaxation', async (t) => {
  const nodes = Array.from({ length: 512 }, (_, i) => node(`n${i}`));
  const data = graph(
    nodes,
    nodes.flatMap(
      (item, i) =>
        [
          [item.id, nodes[(i + 1) % nodes.length]?.id ?? ''],
          [item.id, nodes[(i + 31) % nodes.length]?.id ?? ''],
        ] as const,
    ),
  );
  const harness = workerHarness();
  t.after(async () => {
    await harness.worker.terminate();
  });
  const initial = await harness.send({ type: 'init', graph: data, version: 1 });
  let latest = initial.messages[0];
  assert.equal(latest?.type, 'layout');
  if (latest?.type !== 'layout') assert.fail('No initial layout');
  const initialSpread = axisSpread(latest.positions);
  assert.ok(
    Math.min(...initialSpread) / Math.max(...initialSpread) > 0.7,
    `Flat initial component: ${initialSpread}`,
  );
  for (let tick = 0; tick < 40; tick++) {
    const next = await harness.send({ type: 'test:advance' });
    latest = next.messages[0];
    if (latest?.type !== 'layout') assert.fail('No layout update');
  }
  if (latest?.type !== 'layout') assert.fail('No final layout');
  const finalSpread = axisSpread(latest.positions);
  assert.ok(
    Math.min(...finalSpread) / Math.max(...finalSpread) > 0.7,
    `Flat relaxed component: ${finalSpread}`,
  );
});

test('initial positions are deterministic across node and edge ordering', () => {
  // Given
  const reordered = {
    ...connectedGraph,
    nodes: [...connectedGraph.nodes].reverse(),
    edges: [...connectedGraph.edges].reverse(),
  };
  // When
  const first = createInitialLayout(connectedGraph);
  const second = createInitialLayout(reordered);
  // Then
  assert.deepEqual(
    coordinates(connectedGraph, first.positions),
    coordinates(reordered, second.positions),
  );
  assert.ok(first.positions.every(Number.isFinite));
});

test('isolates stay peripheral and never alter connected positions or force participants', () => {
  // Given
  const extended = {
    ...connectedGraph,
    nodes: [node('isolated-1'), ...connectedGraph.nodes, node('isolated-2')],
  };
  const original = createInitialLayout(connectedGraph);
  // When
  const result = createInitialLayout(extended);
  // Then
  const positions = coordinates(extended, result.positions);
  for (const [id, point] of coordinates(connectedGraph, original.positions))
    assert.deepEqual(positions.get(id), point);
  assert.equal(result.positions.length, extended.nodes.length * 3);
  assert.equal(result.connected.length, connectedGraph.nodes.length);
  assert.ok(result.connected.every((item) => !item.id.startsWith('isolated')));
  const maximum = Math.max(
    ...original.connected.map((item) => Math.hypot(item.centerX, item.centerY) + item.radius),
  );
  for (const id of ['isolated-1', 'isolated-2']) {
    const point = positions.get(id) ?? [];
    assert.ok(Math.hypot(point[0] ?? 0, point[1] ?? 0) > maximum + 80);
  }
});

test('component packing reserves disjoint space and keeps high-degree roots central', () => {
  // Given
  // When
  const layout = createInitialLayout(connectedGraph);
  // Then
  const first = layout.connected.find((item) => item.id === 'b');
  const second = layout.connected.find((item) => item.id === 'd');
  assert.ok(first && second);
  assert.equal(first.x, first.centerX);
  assert.equal(first.y, first.centerY);
  assert.ok(
    Math.hypot(first.centerX - second.centerX, first.centerY - second.centerY) >=
      first.radius + second.radius + 80,
  );
  for (const item of layout.connected)
    assert.ok(Math.hypot(item.x - item.centerX, item.y - item.centerY) <= item.radius + 0.001);
});

test('search ranks exact domain and name above prefix and substring matches with stable limits', () => {
  // Given
  const data = graph(
    [
      node('contains', 'Other', 'my-alpha.example'),
      node('prefix', 'Alphabet'),
      node('name', 'Alpha', 'z.example'),
      node('domain', 'Other', 'alpha'),
      node('neighbor'),
    ],
    [
      ['contains', 'neighbor'],
      ['prefix', 'neighbor'],
    ],
  );
  const search = createGraphSearch(data);
  // When
  const page = search(' ALPHA ', 2);
  const expanded = search('alpha', 4);
  // Then
  assert.deepEqual(page, { ids: ['domain', 'name'], total: 4 });
  assert.deepEqual(expanded, { ids: ['domain', 'name', 'prefix', 'contains'], total: 4 });
  assert.deepEqual(
    createGraphSearch({ ...data, nodes: [...data.nodes].reverse() })('alpha', 4),
    expanded,
  );
  assert.deepEqual(search('not-found', 10), { ids: [], total: 0 });
});

test('empty search lists connected nodes by degree and preserves every match when expanded', () => {
  // Given
  const data = { ...connectedGraph, nodes: [node('00-isolate'), ...connectedGraph.nodes] };
  const search = createGraphSearch(data);
  // When
  const preview = search('', 2);
  // Then
  assert.deepEqual(preview, { ids: ['b', 'a'], total: 6 });
  assert.equal(search('', 6).ids.at(-1), '00-isolate');
  assert.equal(search('', 6).ids.length, 6);
  assert.deepEqual(search('', 0), { ids: [], total: 6 });
});

test('layout pause clears pending work and resume excludes paused time from compute budget', () => {
  // Given
  const { timer, updates, loop } = scheduledLayout(1);
  loop.resume();
  timer.next();
  // When
  loop.pause();
  timer.advance(50_000);
  loop.resume();
  timer.next();
  // Then
  assert.deepEqual(
    updates.map((item) => item.progress),
    [1 / 40, 2 / 40],
  );
  assert.equal(timer.pending, 1);
  loop.cancel();
  loop.resume();
  assert.equal(timer.pending, 0);
});

for (const [cost, count] of [
  [0, 40],
  [1_000, 5],
]) {
  test(`layout yields individual ticks and stops after ${count} throttled updates`, () => {
    // Given
    const { timer, updates, loop } = scheduledLayout(cost);
    // When
    loop.resume();
    while (timer.next()) {
      /* Drain one scheduled task at a time. */
    }
    // Then
    assert.equal(updates.length, count);
    assert.equal(updates.at(-1)?.progress, 1);
    assert.ok(
      updates.every((update, index) => update.time - (updates[index - 1]?.time ?? 0) >= 100),
    );
    loop.resume();
    assert.equal(timer.pending, 0);
  });
}

test('layout worker transfers complete initial buffers and discards superseded versions', async (t) => {
  // Given
  const harness = workerHarness();
  t.after(async () => {
    await harness.worker.terminate();
  });
  const initial = await harness.send({ type: 'init', graph: connectedGraph, version: 1 });
  assert.equal(initial.messages[0]?.type, 'layout');
  if (initial.messages[0]?.type === 'layout') {
    assert.equal(initial.messages[0].positions.length, connectedGraph.nodes.length * 3);
    assert.equal(initial.messages[0].progress, 0);
  }
  const paused = await harness.send({ type: 'pause', version: 1 });
  assert.equal(paused.pending, 0);
  await harness.send({ type: 'resume', version: 1 });
  // When
  await harness.send({ type: 'init', graph: connectedGraph, version: 2 });
  const stale = await harness.send({ type: 'init', graph: connectedGraph, version: 1 });
  const oldPause = await harness.send({ type: 'pause', version: 1 });
  const next = await harness.send({ type: 'test:advance' });
  // Then
  assert.deepEqual(stale.messages, []);
  assert.equal(oldPause.pending, 1);
  assert.equal(next.messages[0]?.version, 2);
  assert.equal(next.messages[0]?.type, 'layout');
  assert.equal((await harness.send({ type: 'cancel', version: 2 })).pending, 0);
  assert.equal((await harness.send({ type: 'resume', version: 2 })).pending, 0);
});

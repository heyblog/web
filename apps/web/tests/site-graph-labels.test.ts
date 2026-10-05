import assert from 'node:assert/strict';
import test from 'node:test';

import {
  createGraphLabels,
  type GraphLabelNode,
  type GraphLabelViewport,
  graphNodeOnScreen,
  graphRectsOverlap,
} from '../src/application/site-graph/site-graph.labels.ts';

const viewport: GraphLabelViewport = {
  clip: { x: 0, y: 0, width: 1200, height: 900 },
  obstacles: [],
  rem: 16,
};
const node = (id: string, x = 400, y = 400, priority = 0): GraphLabelNode => ({
  id,
  x,
  y,
  priority,
  radius: 9,
  persistent: priority >= 3,
});
const nodes = (count: number) =>
  Array.from({ length: count }, (_, i) =>
    node(String(i), 120 + (i % 5) * 230, 100 + Math.floor(i / 5) * 150),
  );

test('wide and narrow views enter and exit automatic labels with hysteresis', () => {
  for (const [width, enter, exit] of [
    [1200, 20, 24],
    [375, 8, 10],
  ]) {
    const layout = createGraphLabels();
    const view = { ...viewport, clip: { ...viewport.clip, width } };
    const crowded = (count: number) =>
      Array.from({ length: count }, (_, i) => node(String(i), 120, 100));
    assert.equal(layout.update([], view).automatic, false);
    assert.equal(layout.update(crowded(enter + 1), view).automatic, false);
    assert.equal(layout.update(crowded(enter), view).automatic, true);
    assert.equal(layout.update(crowded(exit), view).automatic, true);
    assert.equal(layout.update(crowded(exit + 1), view).automatic, false);
    assert.equal(layout.update(crowded(enter + 1), view).automatic, false);
  }
});

test('only nodes inside the clipped browser viewport and outside overlays count', () => {
  const view = {
    ...viewport,
    clip: { x: 100, y: 80, width: 500, height: 500 },
    obstacles: [{ x: 100, y: 400, width: 500, height: 180 }],
  };
  const result = createGraphLabels().update(
    [
      node('shown', 200, 200),
      node('scrolled above', 200, 50),
      node('panel', 200, 450),
      node('right', 620, 200),
      node('invalid', NaN, 200),
    ],
    view,
  );
  assert.equal(result.count, 1);
  assert.deepEqual(
    result.labels.map((label) => label.id),
    ['shown'],
  );
  assert.equal(
    graphNodeOnScreen(node('outside'), { ...view, clip: { ...view.clip, height: 0 } }),
    false,
  );
});

test('sparse nodes show information without selecting them; dense scenes retain priority details', () => {
  const layout = createGraphLabels();
  const sparse = layout.update(nodes(7), viewport);
  assert.equal(sparse.labels.length, 7);
  assert.ok(sparse.labels.every((label) => label.y + label.height < label.nodeY));
  const dense = layout.update([...nodes(30), node('selected', 500, 450, 3)], viewport);
  assert.equal(dense.automatic, false);
  assert.deepEqual(
    dense.labels.map((label) => label.id),
    ['selected'],
  );
});

test('colliding labels are displaced or hidden, with hover before selection and ordinary nodes', () => {
  const result = createGraphLabels().update(
    [
      node('ordinary', 400, 400),
      node('selected', 405, 400, 3),
      node('hover', 410, 400, 4),
      ...Array.from({ length: 4 }, (_, i) => node(`crowded-${i}`, 400, 400)),
    ],
    viewport,
  );
  assert.equal(result.labels[0]?.id, 'hover');
  assert.ok(result.labels.some((label) => label.id === 'selected'));
  assert.ok(result.labels.length < 7);
  for (const a of result.labels)
    for (const b of result.labels)
      if (a.id !== b.id) assert.equal(graphRectsOverlap(a, b, 4), false);
});

test('labels avoid toolbar rectangles and ordinary labels do not cover nearby nodes', () => {
  const view = { ...viewport, obstacles: [{ x: 450, y: 315, width: 250, height: 70 }] };
  const points = [node('a', 400, 400), node('b', 400, 340)];
  const result = createGraphLabels().update(points, view);
  assert.ok(result.labels.length > 0);
  for (const label of result.labels) {
    for (const obstacle of view.obstacles)
      assert.equal(graphRectsOverlap(label, obstacle, 4), false);
    for (const p of points)
      assert.equal(
        graphRectsOverlap(
          label,
          {
            x: p.x - p.radius,
            y: p.y - p.radius,
            width: p.radius * 2,
            height: p.radius * 2,
          },
          4,
        ),
        false,
      );
  }
});

test('a selected node at the top uses a readable label below; labels stay inside narrow clips', () => {
  const view = { ...viewport, clip: { x: 0, y: 0, width: 375, height: 600 } };
  const result = createGraphLabels().update(
    [node('top-left', 10, 10, 3), node('right', 370, 200)],
    view,
  );
  assert.equal(result.labels.length, 2);
  assert.ok((result.labels[0]?.y ?? 0) > 10);
  for (const label of result.labels) {
    assert.ok(label.x >= 4);
    assert.ok(label.x + label.width <= 371);
  }
});

test('placement is stable across input reordering and separate graphs never share hysteresis', () => {
  const layout = createGraphLabels();
  const items = [node('a', 400, 400), node('b', 405, 400), node('c', 410, 400)];
  const before = layout.update(items, viewport);
  assert.deepEqual(layout.update(items.toReversed(), viewport), before);
  assert.equal(layout.update(nodes(21), viewport).automatic, true);
  assert.equal(createGraphLabels().update(nodes(21), viewport).automatic, false);
});

test('the breakpoint and label size scale with root font size', () => {
  const view = { ...viewport, rem: 20, clip: { ...viewport.clip, width: 800 } };
  assert.equal(
    createGraphLabels().update(
      Array.from({ length: 9 }, (_, i) => node(String(i))),
      view,
    ).automatic,
    false,
  );
  const result = createGraphLabels().update([node('readable')], view);
  assert.equal(result.labels[0]?.width, 200);
  assert.equal(result.labels[0]?.height, 60);
});

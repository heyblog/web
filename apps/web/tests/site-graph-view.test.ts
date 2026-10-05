import assert from 'node:assert/strict';
import test from 'node:test';

import { graphFrame } from '../src/application/site-graph/site-graph.view.ts';

test('graph framing centers offset layouts and contains every node in narrow and wide views', () => {
  const positions = new Float32Array([200, -20, 10, 260, 40, 70, 240, 10, 40]);
  for (const aspect of [0.5, 1, 2]) {
    const frame = graphFrame(positions, aspect);
    assert.deepEqual(frame.center, [230, 10, 40]);
    const halfAngle = Math.min(
      Math.atan(Math.tan((25 * Math.PI) / 180) * aspect),
      (25 * Math.PI) / 180,
    );
    assert.ok(frame.distance * Math.sin(halfAngle) > Math.sqrt(3 * 30 ** 2));
  }
  assert.ok(graphFrame(positions, 0.5).distance > graphFrame(positions, 2).distance);
});

test('focus framing includes the chosen neighborhood without distant unrelated nodes', () => {
  const positions = new Float32Array([0, 0, 0, 20, 0, 0, 10_000, 0, 0]);
  const focused = graphFrame(positions, 1, new Set([0, 1]));
  assert.deepEqual(focused.center, [10, 0, 0]);
  assert.ok(focused.distance < graphFrame(positions, 1).distance);
  assert.ok(Number.isFinite(graphFrame(new Float32Array(), 1).distance));
});

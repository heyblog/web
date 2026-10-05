import assert from 'node:assert/strict';
import test from 'node:test';

import { PerspectiveCamera, Vector3 } from 'three';

import { graphAnchor, graphPanOffset } from '../src/application/site-graph/site-graph.camera.ts';
import {
  graphLineOpacity,
  graphNodeSize,
  graphZoomLevel,
} from '../src/application/site-graph/site-graph.navigation.ts';
import { graphFrame } from '../src/application/site-graph/site-graph.view.ts';

test('100 CSS pixels of pan stays 100 pixels across zoom, viewports, depth and orientation', () => {
  for (const [width, height] of [
    [375, 600],
    [1024, 768],
    [1920, 1080],
  ]) {
    for (const zoom of [0.25, 1, 8, 64, 256, 1024]) {
      for (const depth of [80, 1400]) {
        const camera = new PerspectiveCamera(50, width / height, 0.1, 100_000);
        camera.position.set(350, 210, 1600);
        camera.lookAt(new Vector3(50, -20, 0));
        camera.zoom = zoom;
        camera.updateProjectionMatrix();
        camera.updateMatrixWorld();
        const anchor = graphAnchor(camera, width, height, depth, {
          x: width * 0.42,
          y: height * 0.4,
        });
        const before = anchor.clone().project(camera);
        const offset = graphPanOffset(camera, height, depth, 100, -70);
        camera.position.add(offset);
        camera.updateMatrixWorld();
        const after = anchor.project(camera);
        assert.ok(Math.abs(((after.x - before.x) * width) / 2 - 100) < 0.001);
        assert.ok(Math.abs(((before.y - after.y) * height) / 2 + 70) < 0.001);
      }
    }
  }
});

test('zoom keeps an off-center pointer anchor stable, including the maximum zoom', () => {
  for (const zoom of [0.25, 8, 64, 256, 1024]) {
    const width = 1024,
      height = 700,
      depth = 1200;
    const camera = new PerspectiveCamera(50, width / height, 0.1, 100_000);
    camera.position.set(0, 0, depth);
    camera.updateMatrixWorld();
    const pointer = { x: 170, y: 260 };
    const anchor = graphAnchor(camera, width, height, depth, pointer);
    camera.zoom = zoom;
    camera.updateProjectionMatrix();
    const changed = graphAnchor(camera, width, height, depth, pointer);
    camera.position.add(anchor.clone().sub(changed));
    camera.updateMatrixWorld();
    const projected = anchor.project(camera);
    assert.ok(Math.abs(((projected.x + 1) * width) / 2 - pointer.x) < 0.001);
    assert.ok(Math.abs(((1 - projected.y) * height) / 2 - pointer.y) < 0.001);
  }
});

test('fit uses actual node radius, avoiding empty bounding-box corners', () => {
  const points = new Float32Array([
    100, 0, 0, -100, 0, 0, 0, 100, 0, 0, -100, 0, 0, 0, 100, 0, 0, -100,
  ]);
  const frame = graphFrame(points, 1);
  const radius = (frame.distance * Math.sin((25 * Math.PI) / 180)) / 1.12;
  assert.equal(radius, 105);
});

test('zoom enlarges nodes while retaining bounded glyphs and faint baseline edges', () => {
  assert.equal(graphZoomLevel(0), 0.25);
  assert.equal(graphZoomLevel(2048), 1024);
  assert.equal(graphZoomLevel(1024 - 1e-10), 1024);
  assert.equal(graphZoomLevel(0.25 + 1e-10), 0.25);
  assert.equal(graphNodeSize(4, 1), 4);
  assert.equal(graphNodeSize(4, 64), 18);
  assert.equal(graphNodeSize(6, 64), 22);
  assert.equal(graphNodeSize(12, 64), 28);
  assert.equal(graphLineOpacity(0.25), 0.06);
  assert.equal(graphLineOpacity(64), 0.14);
});

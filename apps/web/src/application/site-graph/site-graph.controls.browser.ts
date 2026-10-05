import { type PerspectiveCamera, Vector3 } from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';

import { graphAnchor, graphPanOffset } from './site-graph.camera.ts';
import { graphInput, type GraphInteraction } from './site-graph.input.browser.ts';
import { type GraphPoint, graphZoomLevel } from './site-graph.navigation.ts';

export function createGraphControls(
  canvas: HTMLCanvasElement,
  camera: PerspectiveCamera,
  callbacks: {
    readonly draw: () => void;
    readonly manual: () => void;
    readonly depth: (point: GraphPoint) => number | undefined;
    readonly selected: () => GraphPoint | undefined;
    readonly pick: (point: GraphPoint, kind: 'hover' | 'select' | 'focus') => void;
    readonly leave: () => void;
    readonly reset: () => void;
    readonly zoomed: (zoom: number) => void;
  },
) {
  let interaction: GraphInteraction = { mode: 'pan', fullscreen: false, touchActive: false };
  const controls = new OrbitControls(camera, canvas);
  // Own input once for both SVG and WebGL; retain OrbitControls' rotation constraints.
  controls.disconnect();
  controls.enableDamping = false;
  controls.enablePan = false;
  controls.enableZoom = false;
  controls.addEventListener('change', callbacks.draw);
  let depth = 1;
  const reference = (point: GraphPoint) =>
    callbacks.depth(point) ??
    Math.max(
      camera.near,
      controls.target.clone().sub(camera.position).dot(camera.getWorldDirection(new Vector3())),
    );
  function pan(dx: number, dy: number): void {
    callbacks.manual();
    const offset = graphPanOffset(camera, canvas.clientHeight, depth, dx, dy);
    camera.position.add(offset);
    controls.target.add(offset);
    controls.update();
    callbacks.draw();
  }
  function zoom(
    factor: number,
    point = callbacks.selected() ?? { x: canvas.clientWidth / 2, y: canvas.clientHeight / 2 },
  ): void {
    callbacks.manual();
    const plane = reference(point);
    const before = graphAnchor(camera, canvas.clientWidth, canvas.clientHeight, plane, point);
    camera.zoom = graphZoomLevel(camera.zoom / factor);
    camera.updateProjectionMatrix();
    const after = graphAnchor(camera, canvas.clientWidth, canvas.clientHeight, plane, point);
    const offset = before.sub(after);
    camera.position.add(offset);
    controls.target.add(offset);
    controls.update();
    callbacks.zoomed(camera.zoom);
    callbacks.draw();
  }
  const input = graphInput(canvas, {
    interaction: () => interaction,
    start: (point) => {
      callbacks.manual();
      depth = reference(point);
    },
    pan,
    rotate: (dx, dy) => {
      callbacks.manual();
      controls.rotateLeft((2 * Math.PI * dx) / canvas.clientHeight);
      controls.rotateUp((2 * Math.PI * dy) / canvas.clientHeight);
      controls.update();
    },
    zoom,
    pick: callbacks.pick,
    leave: callbacks.leave,
    reset: callbacks.reset,
  });
  return {
    controls,
    zoom,
    configure(next: GraphInteraction): void {
      interaction = next;
      canvas.style.touchAction = next.touchActive ? 'none' : 'pan-y pinch-zoom';
      canvas.style.cursor = next.mode === 'pan' ? 'grab' : 'move';
    },
    dispose(): void {
      input.destroy();
      controls.dispose();
    },
  };
}

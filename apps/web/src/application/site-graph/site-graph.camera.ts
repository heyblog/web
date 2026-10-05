import { type PerspectiveCamera, Vector3 } from 'three';

import type { GraphPoint } from './site-graph.navigation.ts';

/** Camera translation that moves the grabbed plane by exactly the CSS-pixel delta. */
export function graphPanOffset(
  camera: PerspectiveCamera,
  height: number,
  depth: number,
  dx: number,
  dy: number,
): Vector3 {
  const unit =
    (2 * Math.max(camera.near, depth) * Math.tan((camera.fov * Math.PI) / 360)) /
    (Math.max(1, height) * camera.zoom);
  return new Vector3()
    .setFromMatrixColumn(camera.matrixWorld, 0)
    .multiplyScalar(-dx * unit)
    .addScaledVector(new Vector3().setFromMatrixColumn(camera.matrixWorld, 1), dy * unit);
}

/** Intersection of a screen ray and a camera-facing plane at the captured depth. */
export function graphAnchor(
  camera: PerspectiveCamera,
  width: number,
  height: number,
  depth: number,
  point: GraphPoint,
): Vector3 {
  const ray = new Vector3((2 * point.x) / width - 1, 1 - (2 * point.y) / height, 0.5)
    .unproject(camera)
    .sub(camera.position)
    .normalize();
  return camera.position
    .clone()
    .addScaledVector(
      ray,
      depth / Math.max(0.001, ray.dot(camera.getWorldDirection(new Vector3()))),
    );
}

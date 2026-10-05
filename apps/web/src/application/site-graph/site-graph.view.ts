/** Fit a bounding sphere within both camera axes, leaving room for node glyphs. */
export function graphFrame(
  positions: Float32Array,
  aspect: number,
  included?: ReadonlySet<number>,
): { readonly center: readonly [number, number, number]; readonly distance: number } {
  const min = [Infinity, Infinity, Infinity];
  const max = [-Infinity, -Infinity, -Infinity];
  for (let index = 0; index < positions.length / 3; index++) {
    if (included && !included.has(index)) continue;
    for (let axis = 0; axis < 3; axis++) {
      const value = positions[index * 3 + axis] ?? 0;
      min[axis] = Math.min(min[axis] ?? Infinity, value);
      max[axis] = Math.max(max[axis] ?? -Infinity, value);
    }
  }
  if (!Number.isFinite(min[0])) return { center: [0, 0, 0], distance: 120 };
  const center = [
    ((min[0] ?? 0) + (max[0] ?? 0)) / 2,
    ((min[1] ?? 0) + (max[1] ?? 0)) / 2,
    ((min[2] ?? 0) + (max[2] ?? 0)) / 2,
  ] as const;
  let radius = 24;
  for (let index = 0; index < positions.length / 3; index++) {
    if (included && !included.has(index)) continue;
    radius = Math.max(
      radius,
      Math.hypot(
        (positions[index * 3] ?? 0) - center[0],
        (positions[index * 3 + 1] ?? 0) - center[1],
        (positions[index * 3 + 2] ?? 0) - center[2],
      ) + 5,
    );
  }
  const vertical = (25 * Math.PI) / 180;
  const horizontal = Math.atan(Math.tan(vertical) * Math.max(aspect, 0.1));
  return { center, distance: (radius * 1.12) / Math.sin(Math.min(vertical, horizontal)) };
}

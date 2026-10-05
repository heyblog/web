import type { SiteGraph } from '../../api/sites/site-graph.types.ts';

export interface LayoutNode {
  readonly id: string;
  readonly graphIndex: number;
  readonly centerX: number;
  readonly centerY: number;
  readonly radius: number;
  // The force simulation owns these coordinates and velocities.
  x: number;
  y: number;
  z: number;
  vx?: number;
  vy?: number;
  vz?: number;
}

export interface InitialLayout {
  readonly positions: Float32Array<ArrayBuffer>;
  readonly connected: LayoutNode[];
}

const goldenAngle = Math.PI * (3 - Math.sqrt(5));
const componentGap = 80;

function compareID(left: string, right: string): number {
  return left < right ? -1 : left > right ? 1 : 0;
}

function componentMembers(graph: SiteGraph): number[][] {
  const indices = new Map(graph.nodes.map((node, index) => [node.id, index]));
  const neighbors = new Map<number, number[]>();
  for (const edge of graph.edges) {
    const source = indices.get(edge.source);
    const target = indices.get(edge.target);
    if (source === undefined || target === undefined) continue;
    if (!neighbors.has(source)) neighbors.set(source, []);
    if (!neighbors.has(target)) neighbors.set(target, []);
    neighbors.get(source)?.push(target);
    neighbors.get(target)?.push(source);
  }
  const priority = (left: number, right: number) =>
    (neighbors.get(right)?.length ?? 0) - (neighbors.get(left)?.length ?? 0) ||
    compareID(graph.nodes[left]?.id ?? '', graph.nodes[right]?.id ?? '');
  const roots = [...neighbors.keys()].sort(priority);
  for (const adjacent of neighbors.values()) adjacent.sort(priority);
  const visited = new Set<number>();
  const components: number[][] = [];
  for (const root of roots) {
    if (visited.has(root)) continue;
    const members = [root];
    visited.add(root);
    for (let cursor = 0; cursor < members.length; cursor++) {
      for (const next of neighbors.get(members[cursor] ?? -1) ?? []) {
        if (visited.has(next)) continue;
        visited.add(next);
        members.push(next);
      }
    }
    components.push(members);
  }
  return components.sort(
    (left, right) => right.length - left.length || priority(left[0] ?? -1, right[0] ?? -1),
  );
}

export function createInitialLayout(graph: SiteGraph): InitialLayout {
  const positions = new Float32Array(graph.nodes.length * 3);
  const components = componentMembers(graph);
  const boxes = components.map((members) => ({
    members,
    radius: Math.max(45, Math.sqrt(members.length) * 16),
  }));
  const shelfWidth = Math.max(
    1,
    ...boxes.map(({ radius }) => radius * 2 + componentGap),
    Math.sqrt(boxes.reduce((area, { radius }) => area + (radius * 2 + componentGap) ** 2, 0)) * 1.3,
  );
  const packed: { members: number[]; radius: number; x: number; y: number }[] = [];
  let x = 0;
  let y = 0;
  let shelfHeight = 0;
  let width = 0;
  for (const box of boxes) {
    const side = box.radius * 2 + componentGap;
    if (x > 0 && x + side > shelfWidth) {
      y += shelfHeight;
      x = 0;
      shelfHeight = 0;
    }
    packed.push({ ...box, x: x + side / 2, y: y + side / 2 });
    x += side;
    width = Math.max(width, x);
    shelfHeight = Math.max(shelfHeight, side);
  }
  const height = y + shelfHeight;
  const connected: LayoutNode[] = [];
  const connectedIndices = new Set<number>();
  let perimeter = 0;
  for (const box of packed) {
    const centerX = box.x - width / 2;
    const centerY = box.y - height / 2;
    perimeter = Math.max(perimeter, Math.hypot(centerX, centerY) + box.radius);
    box.members.forEach((graphIndex, order) => {
      // Independent azimuth/elevation sequences fill a volume without correlating depth
      // with the degree-ordered radius. The highest-degree root stays at the center.
      const radius = box.radius * Math.cbrt(order / Math.max(1, box.members.length - 1));
      const angle = order * goldenAngle;
      const vertical = 1 - 2 * ((order * Math.SQRT2) % 1);
      const horizontal = Math.sqrt(1 - vertical * vertical);
      const node: LayoutNode = {
        id: graph.nodes[graphIndex]?.id ?? '',
        graphIndex,
        centerX,
        centerY,
        radius: box.radius,
        x: centerX + Math.cos(angle) * horizontal * radius,
        y: centerY + Math.sin(angle) * horizontal * radius,
        z: vertical * radius,
      };
      connected.push(node);
      connectedIndices.add(graphIndex);
      positions.set([node.x, node.y, node.z], graphIndex * 3);
    });
  }
  const isolated = graph.nodes
    .map((node, index) => ({ id: node.id, index }))
    .filter(({ index }) => !connectedIndices.has(index))
    .sort((left, right) => compareID(left.id, right.id));
  isolated.forEach(({ index }, order) => {
    const radius = perimeter + 85 + Math.sqrt(order) * 12;
    const angle = order * goldenAngle;
    positions.set(
      [Math.cos(angle) * radius, Math.sin(angle) * radius, Math.sin(angle) * 8],
      index * 3,
    );
  });
  return { positions, connected };
}

interface LayoutLoopOptions {
  readonly now: () => number;
  readonly schedule: (callback: () => void, delay: number) => number;
  readonly clear: (timer: number) => void;
  readonly step: () => void;
  readonly publish: (progress: number) => void;
  readonly error: () => void;
}

export function createLayoutLoop(options: LayoutLoopOptions) {
  let state: 'paused' | 'running' | 'done' = 'paused';
  let timer: number | undefined;
  let ticks = 0;
  let computeTime = 0;
  function stop() {
    if (timer !== undefined) options.clear(timer);
    timer = undefined;
  }
  function step() {
    timer = undefined;
    if (state !== 'running') return;
    const start = options.now();
    try {
      options.step();
      computeTime += Math.max(0, options.now() - start);
      ticks++;
      const progress = Math.min(1, Math.max(ticks / 40, computeTime / 5_000));
      if (progress === 1) state = 'done';
      options.publish(progress);
      if (state === 'running') timer = options.schedule(step, 100);
    } catch {
      // no-excuse-ok: catch -- asynchronous worker failures reach the owning boundary.
      state = 'done';
      options.error();
    }
  }
  return {
    pause() {
      if (state === 'running') {
        state = 'paused';
        stop();
      }
    },
    resume() {
      if (state === 'paused') {
        state = 'running';
        timer = options.schedule(step, 100);
      }
    },
    cancel() {
      state = 'done';
      stop();
    },
  };
}

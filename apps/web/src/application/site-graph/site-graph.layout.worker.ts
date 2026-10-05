import { forceLink, forceManyBody, forceSimulation } from 'd3-force-3d';

import { createInitialLayout, createLayoutLoop, type LayoutNode } from './site-graph.layout.ts';
import type { GraphLayoutRequest, GraphLayoutResponse } from './site-graph.protocol.ts';

let version = -1;
let loop: ReturnType<typeof createLayoutLoop> | undefined;

function respond(message: GraphLayoutResponse): void {
  if (message.type === 'layout')
    self.postMessage(message, { transfer: [message.positions.buffer] });
  else self.postMessage(message);
}

self.onmessage = (event: MessageEvent<GraphLayoutRequest>) => {
  const message = event.data;
  try {
    switch (message.type) {
      case 'init': {
        if (message.version <= version) return;
        version = message.version;
        loop?.cancel();
        const layout = createInitialLayout(message.graph);
        respond({
          type: 'layout',
          positions: layout.positions.slice(),
          progress: layout.connected.length ? 0 : 1,
          version,
        });
        if (!layout.connected.length) {
          loop = undefined;
          return;
        }
        const simulation = forceSimulation(layout.connected, 3)
          .stop()
          .alphaDecay(0.08)
          .force('charge', forceManyBody().strength(-18).distanceMax(120))
          .force(
            'link',
            forceLink<LayoutNode, { source: string; target: string }>(
              message.graph.edges.map((edge) => ({ source: edge.source, target: edge.target })),
            )
              .id((node) => node.id)
              .distance(32)
              .strength(0.08),
          )
          .force('component', (alpha: number) => {
            for (const node of layout.connected) {
              node.vx = (node.vx ?? 0) + (node.centerX - node.x) * alpha * 0.01;
              node.vy = (node.vy ?? 0) + (node.centerY - node.y) * alpha * 0.01;
              node.vz = (node.vz ?? 0) - node.z * alpha * 0.01;
            }
          });
        const currentVersion = version;
        loop = createLayoutLoop({
          now: () => performance.now(),
          schedule: (callback, delay) => self.setTimeout(callback, delay),
          clear: (timer) => self.clearTimeout(timer),
          step: () => {
            simulation.tick();
            for (const node of layout.connected) {
              const dx = node.x - node.centerX;
              const dy = node.y - node.centerY;
              const scale = Math.min(1, node.radius / Math.max(1, Math.hypot(dx, dy, node.z)));
              node.x = node.centerX + dx * scale;
              node.y = node.centerY + dy * scale;
              node.z *= scale;
              layout.positions.set([node.x, node.y, node.z], node.graphIndex * 3);
            }
          },
          publish: (progress) => {
            if (currentVersion === version)
              respond({ type: 'layout', positions: layout.positions.slice(), progress, version });
          },
          error: () => respond({ type: 'error', version: currentVersion }),
        });
        loop.resume();
        break;
      }
      case 'pause':
        if (message.version === version) loop?.pause();
        break;
      case 'resume':
        if (message.version === version) loop?.resume();
        break;
      case 'cancel':
        if (message.version === version) loop?.cancel();
        break;
      default:
        message satisfies never;
    }
  } catch {
    // no-excuse-ok: catch -- only a sanitized layout failure leaves the worker.
    respond({ type: 'error', version: message.version });
  }
};

declare module 'd3-force-3d' {
  import type { Simulation, SimulationNodeDatum as Node2D } from 'd3-force';
  export { forceLink, forceManyBody } from 'd3-force';
  export interface SimulationNodeDatum extends Node2D {
    z?: number;
    vz?: number;
  }
  export function forceSimulation<Node extends SimulationNodeDatum>(
    nodes: Node[],
    dimensions?: number,
  ): Simulation<Node, undefined>;
}

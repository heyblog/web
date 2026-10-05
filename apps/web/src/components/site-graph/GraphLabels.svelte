<script lang="ts">
  import type { SiteGraph } from '@/api/sites/site-graph.types';
  import { observeGraphLabelViewport } from '@/application/site-graph/site-graph.label-viewport.browser';
  import {
    createGraphLabels,
    type GraphLabel,
    type GraphLabelNode,
    type GraphLabelViewport,
  } from '@/application/site-graph/site-graph.labels';

  interface Props {
    graph: SiteGraph;
    nodes: readonly GraphLabelNode[];
  }
  let { graph, nodes }: Props = $props();
  const index = $derived(new Map(graph.nodes.map((node) => [node.id, node])));
  const layout = createGraphLabels();
  let viewport = $state.raw<GraphLabelViewport>();
  let labels = $state.raw<readonly GraphLabel[]>([]);
  $effect(() => {
    if (viewport) labels = layout.update(nodes, viewport).labels;
  });
  function measure(host: HTMLElement) {
    return observeGraphLabelViewport(host, (value) => {
      viewport = value;
    });
  }
</script>

<div
  class="pointer-events-none absolute inset-0 z-10 overflow-hidden"
  data-graph-labels
  use:measure
  aria-hidden="true"
>
  {#each labels as label (label.id)}
    {@const node = index.get(label.id)}
    {#if node}<div
        data-graph-label={node.id}
        class="absolute flex flex-col justify-center rounded-md border border-line bg-surface px-2 text-xs shadow-xs"
        style:left={`${label.x}px`}
        style:top={`${label.y}px`}
        style:width={`${label.width}px`}
        style:height={`${label.height}px`}
      >
        <span class="block truncate font-semibold text-fg">{node.name}</span>
        <span class="block truncate text-fg-muted">{node.host}</span>
      </div>{/if}
  {/each}
</div>

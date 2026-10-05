<script lang="ts">
  import { onMount } from 'svelte';

  import type { SiteGraph } from '@/api/sites/site-graph.types';
  import type { GraphInteraction } from '@/application/site-graph/site-graph.input.browser';
  import type { GraphLabelNode } from '@/application/site-graph/site-graph.labels';
  import type { GraphMode } from '@/application/site-graph/site-graph.model';
  import type { GraphRenderer } from '@/application/site-graph/site-graph.renderer.browser';

  import GraphLabels from './GraphLabels.svelte';
  import GraphTools from './GraphTools.svelte';

  interface Props {
    graph: SiteGraph;
    positions: Float32Array;
    selected: string;
    path: readonly string[];
    visible: ReadonlySet<string>;
    active: boolean;
    interaction: GraphInteraction;
    locate: { readonly id: string; readonly request: number };
    frameRequest: number;
    onmode: (mode: GraphMode) => void;
    onselect: (id: string) => void;
    onfocus: (id: string) => void;
    onerror: () => void;
  }
  let {
    graph,
    positions,
    selected,
    path,
    visible,
    active,
    interaction,
    locate,
    frameRequest,
    onmode,
    onselect,
    onfocus,
    onerror,
  }: Props = $props();
  let host: HTMLDivElement;
  let renderer = $state<GraphRenderer>();
  let labels = $state.raw<readonly GraphLabelNode[]>([]);
  let zoom = $state(1);
  let located = -1;
  onMount(() => {
    let disposed = false;
    void import('@/application/site-graph/site-graph.renderer.browser')
      .then(({ createGraphRenderer }) => {
        if (disposed) return;
        try {
          renderer = createGraphRenderer(host, graph, {
            select: onselect,
            focus: onfocus,
            zoom: (value) => {
              zoom = value;
            },
            labels: (next) => {
              labels = next;
            },
          });
        } catch {
          onerror();
        }
      })
      .catch(onerror);
    host.addEventListener('graph-render-error', onerror);
    return () => {
      disposed = true;
      host.removeEventListener('graph-render-error', onerror);
      renderer?.dispose();
    };
  });
  $effect(() => {
    renderer?.select(selected, path, visible);
  });
  $effect(() => {
    if (positions.length) renderer?.layout(positions);
  });
  $effect(() => {
    renderer?.configure(interaction);
  });
  $effect(() => {
    renderer?.setActive(active);
  });
  $effect(() => {
    void frameRequest;
    renderer?.reset();
  });
  $effect(() => {
    if (renderer && positions.length && locate.request !== located) {
      if (locate.id) renderer.focus(locate.id);
      located = locate.request;
    }
  });
</script>

<div class="relative h-full min-h-0 min-w-0">
  <div bind:this={host} class="absolute inset-0 overflow-hidden"></div>
  <GraphLabels {graph} nodes={labels} />
  <GraphTools
    {zoom}
    mode={interaction.mode}
    {onmode}
    onzoom={(factor) => renderer?.zoom(factor)}
    onreset={() => renderer?.reset()}
  />
</div>

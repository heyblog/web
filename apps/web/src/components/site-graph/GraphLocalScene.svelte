<script lang="ts">
  import type { SiteGraph } from '@/api/sites/site-graph.types';
  import type { GraphInteraction } from '@/application/site-graph/site-graph.input.browser';
  import { graphInput } from '@/application/site-graph/site-graph.input.browser';
  import { indexGraph, localGraphLayout } from '@/application/site-graph/site-graph.model';
  import {
    graphLineOpacity,
    graphNodeSize,
    type GraphPoint,
    graphZoomLevel,
  } from '@/application/site-graph/site-graph.navigation';

  import GraphLabels from './GraphLabels.svelte';
  import GraphTools from './GraphTools.svelte';

  interface Props {
    graph: SiteGraph;
    selected: string;
    visible: ReadonlySet<string>;
    interaction: GraphInteraction;
    locate: { readonly id: string; readonly request: number };
    onselect: (id: string) => void;
    onfocus: (id: string) => void;
  }
  let { graph, selected, visible, interaction, locate, onselect, onfocus }: Props = $props();
  const index = $derived(indexGraph(graph));
  const uid = $props.id();
  const positions = $derived(localGraphLayout(graph));
  const points = $derived(
    new Map(
      graph.nodes.map((node, i) => [
        node.id,
        { x: positions[i * 3] ?? 0, y: positions[i * 3 + 1] ?? 0 },
      ]),
    ),
  );
  const edges = $derived(
    graph.edges.filter(
      (edge) =>
        visible.has(edge.source) &&
        visible.has(edge.target) &&
        (!edge.reciprocal || edge.source < edge.target),
    ),
  );
  let width = $state(800);
  let height = $state(480);
  let cx = $state(0);
  let cy = $state(0);
  let scale = $state(1);
  let hover = $state('');
  let fitScale = $state(1);
  const zoomLevel = $derived(scale / fitScale);
  const focus = $derived(hover || (selected === graph.centerId ? '' : selected));
  const related = $derived(index.adjacent.get(focus));
  let located = -1;
  const shown = $derived(graph.nodes.filter((node) => visible.has(node.id)));
  const labels = $derived(
    shown.flatMap((node) => {
      const point = points.get(node.id);
      if (!point) return [];
      return [
        {
          id: node.id,
          x: (point.x - cx) * scale + width / 2,
          y: (point.y - cy) * scale + height / 2,
          radius:
            graphNodeSize(
              node.id === selected || node.id === hover
                ? 12
                : related?.has(node.id) || node.id === graph.centerId
                  ? 6
                  : 4,
              zoomLevel,
            ) / 2,
          priority:
            node.id === hover ? 4 : node.id === selected ? 3 : node.id === graph.centerId ? 1 : 0,
          persistent: node.id === hover || node.id === selected || node.id === graph.centerId,
        },
      ];
    }),
  );
  function fit(id?: string): void {
    let minX = Infinity,
      maxX = -Infinity,
      minY = Infinity,
      maxY = -Infinity;
    const included = id ? new Set([id, ...(index.adjacent.get(id) ?? [])]) : visible;
    for (const node of shown) {
      if (!included.has(node.id)) continue;
      const p = points.get(node.id);
      if (!p) continue;
      minX = Math.min(minX, p.x);
      maxX = Math.max(maxX, p.x);
      minY = Math.min(minY, p.y);
      maxY = Math.max(maxY, p.y);
    }
    if (!Number.isFinite(minX)) return;
    cx = (minX + maxX) / 2;
    cy = (minY + maxY) / 2;
    fitScale = Math.min(
      Math.max(48, width - 96) / Math.max(48, maxX - minX),
      Math.max(48, height - 144) / Math.max(48, maxY - minY),
    );
    scale = fitScale;
  }
  function zoom(factor: number, anchor?: GraphPoint): void {
    const selectedPoint = points.get(selected);
    const projected = selectedPoint && {
      x: (selectedPoint.x - cx) * scale + width / 2,
      y: (selectedPoint.y - cy) * scale + height / 2,
    };
    const p =
      anchor ??
      (projected &&
      projected.x >= 0 &&
      projected.x <= width &&
      projected.y >= 0 &&
      projected.y <= height
        ? projected
        : { x: width / 2, y: height / 2 });
    const worldX = cx + (p.x - width / 2) / scale;
    const worldY = cy + (p.y - height / 2) / scale;
    scale = fitScale * graphZoomLevel(zoomLevel / factor);
    cx = worldX - (p.x - width / 2) / scale;
    cy = worldY - (p.y - height / 2) / scale;
  }
  $effect(() => {
    if (locate.request !== located) {
      if (points.has(locate.id)) fit(locate.id);
      located = locate.request;
    }
  });
  function viewport(svg: SVGSVGElement) {
    let initialized = false;
    const observer = new ResizeObserver(() => {
      if (!svg.clientWidth || !svg.clientHeight) return;
      width = svg.clientWidth;
      height = svg.clientHeight;
      if (!initialized) {
        fit(locate.id || undefined);
        initialized = true;
      }
    });
    observer.observe(svg);
    function nearest(x: number, y: number): string {
      let distance = 22;
      let id = '';
      for (const node of shown) {
        const p = points.get(node.id);
        if (!p) continue;
        const d = Math.hypot(
          (p.x - cx) * scale + width / 2 - x,
          (p.y - cy) * scale + height / 2 - y,
        );
        if (d < distance) {
          distance = d;
          id = node.id;
        }
      }
      return id;
    }
    const gesture = graphInput(svg, {
      interaction: () => interaction,
      start: () => {
        hover = '';
      },
      rotate: () => {},
      leave: () => {
        hover = '';
      },
      pan: (x, y) => {
        cx -= x / scale;
        cy -= y / scale;
      },
      zoom,
      pick: (point, kind) => {
        const id = nearest(point.x, point.y);
        if (kind === 'hover') hover = id;
        else if (id) {
          if (kind === 'focus') onfocus(id);
          else onselect(id);
        }
      },
      reset: () => fit(),
    });
    return {
      destroy() {
        observer.disconnect();
        gesture.destroy();
      },
    };
  }
</script>

<div class="relative h-full min-h-0 min-w-0 overflow-hidden">
  <svg
    use:viewport
    viewBox={`${cx - width / scale / 2} ${cy - height / scale / 2} ${width / scale} ${height / scale}`}
    class="size-full cursor-grab"
    style:touch-action={interaction.touchActive ? 'none' : 'pan-y pinch-zoom'}
    role="group"
    tabindex="0"
    aria-label="博客直接友链关系，方向键平移，加减键缩放，Home 适配视图"
  >
    <defs
      ><marker
        id={`${uid}-arrow`}
        viewBox="0 0 10 10"
        refX="9"
        refY="5"
        markerUnits="userSpaceOnUse"
        markerWidth={6 / scale}
        markerHeight={6 / scale}
        orient="auto-start-reverse"
        ><path d="M 0 0 L 10 5 L 0 10 z" fill="var(--sem-fg-muted)" /></marker
      ></defs
    >
    {#each edges as edge (`${edge.source}:${edge.target}`)}
      {@const a = points.get(edge.source)}{@const b = points.get(edge.target)}
      {#if a && b}
        {@const emphasized = edge.source === focus || edge.target === focus}
        {@const length = Math.max(1, Math.hypot(b.x - a.x, b.y - a.y))}
        {@const inset = Math.min(12 / scale / length, 0.3)}
        <line
          x1={a.x + (b.x - a.x) * inset}
          y1={a.y + (b.y - a.y) * inset}
          x2={b.x - (b.x - a.x) * inset}
          y2={b.y - (b.y - a.y) * inset}
          stroke={edge.reciprocal ? 'var(--sem-success-fg)' : 'var(--sem-fg-muted)'}
          stroke-width={emphasized ? 1.25 : 1}
          stroke-opacity={emphasized ? 0.35 : focus ? 0.03 : graphLineOpacity(zoomLevel)}
          vector-effect="non-scaling-stroke"
          marker-start={emphasized && edge.reciprocal ? `url(#${uid}-arrow)` : undefined}
          marker-end={emphasized ? `url(#${uid}-arrow)` : undefined}
          pointer-events="none"
        />
      {/if}
    {/each}
    {#each shown as node (node.id)}
      {@const point = points.get(node.id)}
      {@const size = graphNodeSize(
        node.id === selected || node.id === hover
          ? 12
          : related?.has(node.id) || node.id === graph.centerId
            ? 6
            : 4,
        zoomLevel,
      )}
      {#if point}<a
          href={`/graph?focus=${encodeURIComponent(node.id)}`}
          role="button"
          aria-label={`${node.name} · ${node.host}`}
          aria-pressed={node.id === selected}
          onclick={(event) => {
            event.preventDefault();
            if (event.detail === 0) onselect(node.id);
          }}
          onkeydown={(event) => {
            if (event.key === ' ') {
              event.preventDefault();
              onselect(node.id);
            }
          }}
        >
          <title>{node.name} · {node.host}</title>
          {#if node.id === selected}<circle
              cx={point.x}
              cy={point.y}
              r={(size / 2 + 3) / scale}
              fill="none"
              stroke="var(--sem-info-fg)"
              stroke-opacity="0.65"
              stroke-width="1"
              vector-effect="non-scaling-stroke"
              pointer-events="none"
            />{/if}
          {#if node.shortId}<circle
              cx={point.x}
              cy={point.y}
              r={size / 2 / scale}
              fill={node.id === selected ? 'var(--sem-info-fg)' : 'var(--sem-tint-fg)'}
              stroke="var(--sem-canvas)"
              stroke-width="1"
              vector-effect="non-scaling-stroke"
            />
          {:else}<path
              d={`M ${point.x} ${point.y - size / 2 / scale} l ${size / 2 / scale} ${size / 2 / scale} l ${-size / 2 / scale} ${size / 2 / scale} l ${-size / 2 / scale} ${-size / 2 / scale} Z`}
              fill={node.id === selected ? 'var(--sem-info-fg)' : 'var(--sem-warning-fg)'}
            />{/if}
        </a>{/if}
    {/each}
  </svg>
  <GraphLabels {graph} nodes={labels} />
  <GraphTools
    zoom={zoomLevel}
    local
    mode="pan"
    onmode={() => {}}
    onzoom={(factor) => zoom(factor)}
    onreset={() => fit()}
  />
</div>

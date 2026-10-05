<script lang="ts">
  import {
    IconArrowsMove,
    IconFocus2,
    IconMinus,
    IconPlus,
    IconRotate3d,
  } from '@tabler/icons-svelte';

  import type { GraphMode } from '@/application/site-graph/site-graph.model';
  import { graphZoom } from '@/application/site-graph/site-graph.navigation';

  interface Props {
    local?: boolean;
    zoom: number;
    mode: GraphMode;
    onmode: (mode: GraphMode) => void;
    onzoom: (factor: number) => void;
    onreset: () => void;
  }
  let { local = false, zoom, mode, onmode, onzoom, onreset }: Props = $props();
  const buttonClasses =
    'inline-flex size-11 items-center justify-center rounded-md text-fg-muted hover:bg-subtle hover:text-fg disabled:cursor-not-allowed disabled:opacity-40 sm:size-10';
</script>

<div
  class="pointer-events-none absolute inset-x-3 bottom-3 flex flex-wrap items-end justify-between gap-2"
  aria-label="图谱视角"
>
  {#if !local}<div
      class="pointer-events-auto flex rounded-md border border-line-strong bg-surface p-1"
      aria-label="拖动方式"
      data-graph-label-obstacle
    >
      <button
        type="button"
        class={[buttonClasses, mode === 'rotate' && 'bg-tint text-tint-fg']}
        aria-label="旋转模式"
        title="旋转"
        aria-pressed={mode === 'rotate'}
        onclick={() => onmode('rotate')}><IconRotate3d size={18} /></button
      >
      <button
        type="button"
        class={[buttonClasses, mode === 'pan' && 'bg-tint text-tint-fg']}
        aria-label="平移模式"
        title="平移"
        aria-pressed={mode === 'pan'}
        onclick={() => onmode('pan')}><IconArrowsMove size={18} /></button
      >
    </div>{:else}<span></span>{/if}
  <div
    class="pointer-events-auto flex items-center rounded-md border border-line-strong bg-surface p-1"
    data-graph-label-obstacle
  >
    <output class="min-w-14 text-center font-mono text-xs text-fg-muted" aria-label="缩放倍数"
      >{Number(zoom.toFixed(2))}×</output
    >
    <button
      type="button"
      class={buttonClasses}
      disabled={zoom >= graphZoom.max}
      aria-label="放大"
      title="放大"
      onclick={() => onzoom(0.8)}><IconPlus size={18} /></button
    >
    <button
      type="button"
      class={buttonClasses}
      disabled={zoom <= graphZoom.min}
      aria-label="缩小"
      title="缩小"
      onclick={() => onzoom(1.25)}><IconMinus size={18} /></button
    >
    <button
      type="button"
      class={buttonClasses}
      aria-label="适配视图"
      title="适配视图"
      onclick={onreset}><IconFocus2 size={18} /></button
    >
  </div>
</div>

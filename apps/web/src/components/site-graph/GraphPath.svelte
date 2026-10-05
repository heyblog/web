<script lang="ts">
  import { IconX } from '@tabler/icons-svelte';

  import type { GraphNode } from '@/api/sites/site-graph.types';
  import type { GraphSelection } from '@/application/site-graph/site-graph.shared';

  interface Props {
    nodes: readonly GraphNode[];
    selection: GraphSelection;
    path: readonly string[] | null | undefined;
    onchange: (next: GraphSelection) => void;
    onselect: (id: string) => void;
  }
  let { nodes, selection, path, onchange, onselect }: Props = $props();
  const lookup = $derived(new Map(nodes.map((node) => [node.id, node])));
  let shown = $derived(Math.min(path?.length ?? 0, 40));
</script>

<section class="border-t border-line bg-surface p-4" aria-label="最短路径">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <h2 class="text-sm font-semibold">最短路径</h2>
    <select
      class="min-h-11 rounded-md border border-line-strong bg-surface px-3 text-sm sm:min-h-10"
      aria-label="路径方向"
      value={selection.direction}
      onchange={(event) =>
        onchange({
          ...selection,
          direction: event.currentTarget.value === 'undirected' ? 'undirected' : 'directed',
        })}
      ><option value="directed">沿友链方向</option><option value="undirected">忽略方向</option
      ></select
    >
  </div>
  <div class="mt-3 grid gap-2 sm:grid-cols-2">
    {#each ['from', 'to'] as key (key)}
      {@const id = key === 'from' ? selection.from : selection.to}
      <div class="flex min-h-11 min-w-0 items-center gap-2 border-b border-line py-2 text-sm">
        <span class="shrink-0 text-fg-muted">{key === 'from' ? '起点' : '终点'}</span><span
          class="min-w-0 flex-1 truncate">{lookup.get(id)?.name ?? '未选择'}</span
        >{#if id}<button
            type="button"
            class="inline-flex size-11 shrink-0 items-center justify-center rounded-sm text-fg-muted hover:bg-subtle sm:size-10"
            aria-label={key === 'from' ? '清除起点' : '清除终点'}
            title={key === 'from' ? '清除起点' : '清除终点'}
            onclick={() => onchange({ ...selection, [key]: '' })}><IconX size={16} /></button
          >{/if}
      </div>
    {/each}
  </div>
  <div class="mt-3" aria-live="polite">
    {#if selection.from && selection.to}
      {#if path === undefined}<p class="text-sm text-fg-muted">正在查找路径…</p>
      {:else if path === null}<p class="text-sm text-fg-muted">
          这两个博客之间暂无{selection.direction === 'directed' ? '有向' : ''}路径。
        </p>
      {:else}<p class="text-sm text-fg-muted">{path.length - 1} 跳</p>
        <ol class="mt-2 flex flex-wrap items-center gap-2">
          {#each path.slice(0, shown) as id, index (id)}<li class="flex min-w-0 items-center gap-2">
              {#if index}<span class="text-fg-muted" aria-hidden="true">→</span>{/if}<button
                type="button"
                class="min-h-11 max-w-64 rounded-sm px-2 text-left text-sm font-medium wrap-anywhere text-tint-fg hover:bg-tint"
                onclick={() => onselect(id)}>{lookup.get(id)?.name}</button
              >
            </li>{/each}
        </ol>
        {#if path.length > shown}<button
            type="button"
            class="min-h-11 px-2 text-sm font-medium text-tint-fg hover:bg-tint"
            onclick={() => (shown += 40)}>加载更多路径节点</button
          >{/if}{/if}
    {:else}<p class="text-sm text-fg-muted">
        选择博客并设为起点和终点，查看它们之间的友链路径。
      </p>{/if}
  </div>
</section>

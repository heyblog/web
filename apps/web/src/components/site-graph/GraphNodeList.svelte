<script lang="ts">
  import type { GraphNode } from '@/api/sites/site-graph.types';

  interface Props {
    nodes: readonly GraphNode[];
    selected: string;
    onselect: (id: string) => void;
    paginated?: boolean;
  }
  let { nodes, selected, onselect, paginated = true }: Props = $props();
  let count = $derived(Math.min(40, nodes.length));
</script>

<ul class="divide-y divide-line">
  {#each paginated ? nodes.slice(0, count) : nodes as node (node.id)}
    <li>
      <button
        type="button"
        class={[
          'flex min-h-11 w-full min-w-0 items-center gap-3 rounded-sm px-3 py-2 text-left hover:bg-subtle',
          selected === node.id && 'bg-tint',
        ]}
        aria-pressed={selected === node.id}
        title={`${node.name} · ${node.host}`}
        onclick={() => onselect(node.id)}
        onkeydown={(event) => {
          const buttons = Array.from(
            event.currentTarget.closest('ul')?.querySelectorAll<HTMLButtonElement>('button') ?? [],
          );
          const current = buttons.indexOf(event.currentTarget);
          const next =
            event.key === 'ArrowDown'
              ? Math.min(current + 1, buttons.length - 1)
              : event.key === 'ArrowUp'
                ? Math.max(current - 1, 0)
                : event.key === 'Home'
                  ? 0
                  : event.key === 'End'
                    ? buttons.length - 1
                    : undefined;
          if (next !== undefined) {
            event.preventDefault();
            buttons[next]?.focus();
          }
        }}
      >
        <span
          class={[
            'size-2 shrink-0',
            node.shortId ? 'rounded-full bg-tint-fg' : 'rotate-45 bg-warning-fg',
          ]}
          aria-hidden="true"
        ></span>
        <span class="min-w-0 flex-1"
          ><span class="block truncate text-sm font-medium text-fg">{node.name}</span><span
            class="block truncate text-xs text-fg-muted">{node.host}</span
          ></span
        >
        {#if !node.shortId}<span class="shrink-0 text-xs text-fg-muted">未收录</span>{/if}
      </button>
    </li>
  {/each}
</ul>
{#if paginated && nodes.length > count}<button
    type="button"
    class="min-h-11 w-full text-sm font-medium text-tint-fg hover:bg-tint"
    onclick={() => (count += 40)}>加载更多</button
  >{/if}

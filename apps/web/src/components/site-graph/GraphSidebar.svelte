<script lang="ts">
  import { IconArrowUpRight, IconFocus2, IconRoute, IconTopologyStar3 } from '@tabler/icons-svelte';

  import type { SiteGraph } from '@/api/sites/site-graph.types';
  import type { GraphIndex, GraphRelation } from '@/application/site-graph/site-graph.model';
  import { graphHref, nodeDetailHref } from '@/application/site-graph/site-graph.shared';
  import { siteOutboundAttributes } from '@/application/site-outbound/site-outbound.shared';

  import GraphNodeList from './GraphNodeList.svelte';

  interface Props {
    graph: SiteGraph;
    index: GraphIndex;
    selected: string;
    local: boolean;
    relation: GraphRelation;
    onrelation: (value: GraphRelation) => void;
    onselect: (id: string) => void;
    onlocate: (id: string) => void;
    onsetfrom: () => void;
    onsetto: () => void;
  }
  let {
    graph,
    index,
    selected,
    local,
    relation,
    onrelation,
    onselect,
    onlocate,
    onsetfrom,
    onsetto,
  }: Props = $props();
  const node = $derived(index.nodes.get(selected)?.node);
  const detail = $derived(node ? nodeDetailHref(node) : null);
  const relations = $derived(index.relations(local ? (graph.centerId ?? selected) : selected));
  const tabs: readonly { key: GraphRelation; label: string }[] = [
    { key: 'all', label: '全部' },
    { key: 'outgoing', label: '出链' },
    { key: 'incoming', label: '入链' },
    { key: 'reciprocal', label: '互链' },
  ];
  const buttonClasses =
    'inline-flex min-h-11 items-center justify-center gap-2 rounded-md border border-line-strong px-3 text-sm font-medium text-fg hover:bg-subtle sm:min-h-10';
</script>

{#if node}
  <section
    class="min-w-0 border-b border-line p-4"
    aria-label="选中的博客"
    data-site-impression={node.shortId ?? undefined}
  >
    <div class="flex items-start justify-between gap-3">
      <h2 class="min-w-0 text-base font-semibold wrap-anywhere">{node.name}</h2>
      {#if !node.shortId}<span
          class="shrink-0 rounded-sm bg-warning-bg px-2 py-1 text-xs text-warning-fg">未收录</span
        >{/if}
    </div>
    <p class="mt-1 text-sm wrap-anywhere text-fg-muted">{node.host}</p>
    <div class="mt-3 flex flex-wrap gap-2">
      <button class={buttonClasses} type="button" onclick={() => onlocate(node.id)}
        ><IconFocus2 size={16} />定位</button
      >
      <a
        class={[buttonClasses, 'bg-tint text-tint-fg']}
        {...siteOutboundAttributes(
          node.shortId ? { shortId: node.shortId } : { url: node.homepageUrl },
        )}>访问博客<IconArrowUpRight size={16} /></a
      >
      {#if detail}<a class={buttonClasses} href={detail}>博客详情</a>{/if}
    </div>
    <div class="mt-2 flex flex-wrap gap-2">
      {#if local}<a class={buttonClasses} href={graphHref({ focus: node.id })}
          ><IconTopologyStar3 size={16} />全局图谱</a
        >
        {#if selected !== graph.centerId && graph.centerId}<button
            type="button"
            class={buttonClasses}
            onclick={() => onlocate(graph.centerId ?? '')}>回到当前博客</button
          >{/if}
      {:else}<button class={buttonClasses} type="button" onclick={onsetfrom}
          ><IconRoute size={16} />设为起点</button
        ><button class={buttonClasses} type="button" onclick={onsetto}>设为终点</button>{/if}
    </div>
  </section>
  <section class="min-w-0" aria-label={local ? '当前博客的直接友链' : '选中博客的友链'}>
    {#if local}<h3 class="px-4 pt-3 text-xs font-medium text-fg-muted">当前博客的友链</h3>{/if}
    <div class="flex border-b border-line px-2" aria-label="友链方向">
      {#each tabs as tab (tab.key)}<button
          type="button"
          class={[
            'min-h-11 min-w-0 flex-1 border-b-2 px-1 py-2 text-xs font-medium',
            relation === tab.key
              ? 'border-primary text-fg'
              : 'border-transparent text-fg-muted hover:text-fg',
          ]}
          aria-pressed={relation === tab.key}
          onclick={() => onrelation(tab.key)}
          >{tab.label}<span class="ml-1 font-mono">{relations[tab.key].length}</span></button
        >{/each}
    </div>
    {#if relations[relation].length}<GraphNodeList
        nodes={relations[relation]}
        {selected}
        {onselect}
      />
    {:else}<p class="p-6 text-center text-sm text-fg-muted">
        {relation === 'all'
          ? '暂无公开友链'
          : `暂无${tabs.find((tab) => tab.key === relation)?.label}`}
      </p>{/if}
  </section>
{:else}<p class="p-6 text-center text-sm text-fg-muted">选择一个博客，查看它的友链。</p>{/if}

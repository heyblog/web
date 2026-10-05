<script lang="ts">
  import {
    IconArrowsMaximize,
    IconArrowsMinimize,
    IconBrowser,
    IconChevronDown,
    IconChevronUp,
    IconHandMove,
    IconList,
    IconRefresh,
    IconSearch,
    IconTopologyStar3,
    IconX,
  } from '@tabler/icons-svelte';
  import { onMount } from 'svelte';

  import { createGraphExplorer } from '@/application/site-graph/site-graph.controller.svelte';
  import {
    graphFullscreen,
    type GraphFullscreenMode,
  } from '@/application/site-graph/site-graph.fullscreen.browser';
  import type { GraphMode } from '@/application/site-graph/site-graph.model';

  import GraphLocalScene from './GraphLocalScene.svelte';
  import GraphNodeList from './GraphNodeList.svelte';
  import GraphPath from './GraphPath.svelte';
  import GraphScene from './GraphScene.svelte';
  import GraphSidebar from './GraphSidebar.svelte';

  interface Props {
    identifier?: string;
    active?: boolean;
  }
  let { identifier, active = true }: Props = $props();
  const explorer = createGraphExplorer(
    () => identifier,
    () => active,
  );
  const local = $derived(Boolean(identifier));
  const uid = $props.id();
  let fullscreenMode = $state<GraphFullscreenMode>('inline');
  const fullscreen = $derived(fullscreenMode !== 'inline');
  let fullscreenMessage = $state('');
  let touchActive = $state(false);
  let previousTouch = false;
  let coarse = $state(false);
  let mode = $state<GraphMode>('pan');
  let panelOpen = $state(true);
  let previousPanel = true;
  let pathOpen = $state(false);
  let renderFailed = $state(false);
  let renderVersion = $state(0);
  let workspace: HTMLDivElement;
  let fullscreenControl: ReturnType<typeof graphFullscreen> | undefined;
  const interaction = $derived({ mode, fullscreen, touchActive: fullscreen || touchActive });
  const selectedNode = $derived(explorer.index?.nodes.get(explorer.selection.focus)?.node);
  const buttonClasses =
    'inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-md border border-line-strong bg-surface px-3 text-sm font-medium text-fg hover:bg-subtle sm:min-h-10';

  function expand(host: HTMLDivElement) {
    fullscreenControl = graphFullscreen(
      host,
      (next) => {
        if (fullscreenMode === 'inline' && next !== 'inline') {
          previousTouch = touchActive;
          touchActive = true;
          previousPanel = panelOpen;
          if (coarse) panelOpen = false;
        } else if (fullscreenMode !== 'inline' && next === 'inline') {
          touchActive = previousTouch;
          if (coarse) panelOpen = previousPanel;
        }
        fullscreenMode = next;
        fullscreenMessage = '';
      },
      (message) => {
        fullscreenMessage = message;
      },
    );
    return { destroy: () => fullscreenControl?.destroy() };
  }
  function search(value: string): void {
    explorer.search(value);
    panelOpen = true;
  }
  function select(id: string): void {
    explorer.select(id);
    panelOpen = true;
  }
  function locate(id: string): void {
    explorer.focus(id);
    if (coarse && fullscreen) panelOpen = false;
  }
  function retryRender(): void {
    renderFailed = false;
    renderVersion++;
  }
  onMount(() => {
    const dispose = explorer.mount();
    const media = window.matchMedia('(any-pointer: coarse)');
    const update = () => {
      coarse = media.matches;
    };
    update();
    panelOpen = !coarse;
    media.addEventListener('change', update);
    return () => {
      dispose();
      media.removeEventListener('change', update);
    };
  });
  $effect(() => {
    if (!active && fullscreen) void fullscreenControl?.close();
  });
</script>

<section class="min-w-0" aria-label={local ? '博客友链图谱' : '全局博客友链图谱'}>
  {#if !local}<header
      class="mx-auto flex w-[min(calc(100%-2rem),80rem)] flex-wrap items-end justify-between gap-4 py-6 sm:w-[min(calc(100%-3rem),80rem)] sm:py-8"
    >
      <h1 class="text-2xl font-semibold">友链图谱</h1>
      {#if explorer.graph}<div class="flex flex-wrap gap-x-6 gap-y-2 text-sm text-fg-muted">
          <span
            ><strong class="font-mono font-medium text-fg"
              >{explorer.graph.stats.nodes.toLocaleString()}</strong
            > 博客</span
          ><span
            ><strong class="font-mono font-medium text-fg"
              >{explorer.graph.stats.edges.toLocaleString()}</strong
            > 友链</span
          ><span
            ><strong class="font-mono font-medium text-fg"
              >{explorer.graph.stats.reciprocalPairs.toLocaleString()}</strong
            > 互链</span
          >
        </div>{/if}
    </header>{/if}
  {#if explorer.loading || explorer.failed}<div
      class="flex min-h-100 flex-col items-center justify-center gap-4 px-4 text-center"
      aria-live="polite"
    >
      <IconTopologyStar3 class="text-fg-muted" size={32} aria-hidden="true" />
      <p class="text-sm text-fg-muted">
        {explorer.failed ? '友链图谱暂时无法加载' : '正在加载友链图谱…'}
      </p>
      {#if explorer.failed}<button
          type="button"
          class={buttonClasses}
          onclick={() => void explorer.load()}><IconRefresh size={16} />重新加载</button
        >{/if}
    </div>
  {:else if explorer.graph && explorer.index}
    {@const graph = explorer.graph}
    <div
      bind:this={workspace}
      use:expand
      data-graph-workspace
      role="region"
      aria-label={local ? '站点图谱工作区' : '全局图谱工作区'}
      class={fullscreen
        ? 'fixed inset-0 z-50 flex h-dvh w-full flex-col overflow-hidden bg-surface pt-[env(safe-area-inset-top)] pr-[env(safe-area-inset-right)] pb-[env(safe-area-inset-bottom)] pl-[env(safe-area-inset-left)]'
        : [
            'relative min-w-0 overflow-hidden rounded-md border border-line bg-surface',
            !local &&
              'mx-auto mb-8 w-[min(calc(100%-2rem),80rem)] sm:w-[min(calc(100%-3rem),80rem)]',
          ]}
    >
      <div
        class="flex shrink-0 flex-wrap items-center gap-2 border-b border-line p-3 sm:gap-3 sm:p-4"
      >
        <div class="relative min-w-0 flex-1 basis-40">
          <IconSearch
            data-graph-label-obstacle
            class="pointer-events-none absolute top-3 left-3 text-fg-muted"
            size={18}
            aria-hidden="true"
          />
          <input
            type="search"
            class="min-h-11 w-full rounded-md border border-line-strong bg-surface px-10 text-sm sm:min-h-10"
            aria-label="搜索博客名称或域名"
            placeholder="搜索博客名称或域名"
            value={explorer.query}
            oninput={(event) => search(event.currentTarget.value)}
            onfocus={() => {
              explorer.setSearching(true);
              panelOpen = true;
            }}
            onkeydown={(event) => {
              if (event.key === 'Escape') {
                explorer.setSearching(false);
              }
              if (event.key === 'ArrowDown') {
                event.preventDefault();
                workspace.querySelector<HTMLElement>('[data-graph-results] button')?.focus();
              }
            }}
          />
          {#if explorer.query}<button
              type="button"
              class="absolute top-0 right-0 inline-flex size-11 items-center justify-center text-fg-muted sm:size-10"
              aria-label="清空搜索"
              onclick={() => search('')}><IconX size={16} /></button
            >{/if}
        </div>
        <button
          type="button"
          class={buttonClasses}
          aria-label={panelOpen ? '收起博客面板' : '展开博客面板'}
          aria-expanded={panelOpen}
          aria-controls={`${uid}-panel`}
          onclick={() => (panelOpen = !panelOpen)}
          ><IconList size={18} /><span class="hidden sm:inline">博客</span></button
        >
        <button
          type="button"
          class={buttonClasses}
          data-graph-fullscreen
          aria-label={fullscreenMode === 'page' ? '退出网页全屏' : '网页全屏'}
          aria-pressed={fullscreenMode === 'page'}
          title={fullscreenMode === 'page' ? '退出网页全屏' : '网页全屏'}
          onclick={() => void fullscreenControl?.togglePage()}
          ><IconBrowser size={18} /><span class="hidden sm:inline"
            >{fullscreenMode === 'page' ? '退出网页全屏' : '网页全屏'}</span
          ></button
        >
        <button
          type="button"
          class={buttonClasses}
          aria-label={fullscreenMode === 'native' ? '退出全屏' : '全屏'}
          aria-pressed={fullscreenMode === 'native'}
          title={fullscreenMode === 'native' ? '退出全屏' : '全屏'}
          onclick={() => void fullscreenControl?.toggleNative()}
          >{#if fullscreenMode === 'native'}<IconArrowsMinimize
              size={18}
            />{:else}<IconArrowsMaximize size={18} />{/if}<span class="hidden sm:inline"
            >{fullscreenMode === 'native' ? '退出全屏' : '全屏'}</span
          ></button
        >
      </div>
      <div
        class="flex shrink-0 flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-line px-3 py-2 text-xs text-fg-muted sm:px-4"
      >
        <div class="flex flex-wrap items-center gap-3">
          <span class="flex items-center gap-1.5"
            ><span class="size-2 rounded-full bg-tint-fg"></span>已收录</span
          ><span class="flex items-center gap-1.5"
            ><span class="size-2 rotate-45 bg-warning-fg"></span>未收录</span
          ><span class="flex items-center gap-1.5"
            ><span class="h-0.5 w-3 bg-success-fg"></span>互链</span
          >
        </div>
        {#if !local}<div class="flex flex-wrap items-center gap-x-4 gap-y-1">
            <label class="inline-flex min-h-10 cursor-pointer items-center gap-2"
              ><input
                type="checkbox"
                role="switch"
                class="peer sr-only"
                checked={explorer.display.isolated}
                onchange={(event) =>
                  explorer.setDisplay({
                    ...explorer.display,
                    isolated: event.currentTarget.checked,
                  })}
              /><span
                class="relative h-5 w-9 rounded-full bg-line-strong transition-colors duration-(--motion-base) peer-checked:bg-primary peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-primary motion-reduce:transition-none"
                aria-hidden="true"
                ><span
                  class={[
                    'absolute top-0.5 left-0.5 size-4 rounded-full bg-surface transition-transform duration-(--motion-base) motion-reduce:transition-none',
                    explorer.display.isolated && 'translate-x-4',
                  ]}
                ></span></span
              >显示孤立博客</label
            >
            <span
              title={!explorer.display.isolated
                ? `已隐藏 ${(graph.nodes.length - explorer.index.connected.size).toLocaleString()} 个孤立博客`
                : undefined}
              >{explorer.visible.size.toLocaleString()} / {graph.nodes.length.toLocaleString()} 博客</span
            >
            {#if explorer.selection.focus}<button
                type="button"
                class="min-h-10 font-medium text-tint-fg"
                aria-pressed={explorer.display.scope === 'neighbors'}
                onclick={() =>
                  explorer.setDisplay({
                    ...explorer.display,
                    scope: explorer.display.scope === 'all' ? 'neighbors' : 'all',
                  })}>{explorer.display.scope === 'neighbors' ? '返回全部' : '只看相关'}</button
              >{/if}
          </div>{:else}<span
            >{explorer.visible.size > 0 ? explorer.visible.size - 1 : 0} 个关联博客</span
          >{/if}
      </div>
      {#if fullscreenMessage}<p
          class="shrink-0 border-b border-line bg-warning-bg px-4 py-2 text-sm text-warning-fg"
          role="status"
        >
          {fullscreenMessage}
        </p>{/if}
      {#if explorer.invalid}<div
          class="flex shrink-0 flex-wrap items-center gap-3 bg-warning-bg px-4 py-2 text-sm text-warning-fg"
          role="status"
        >
          地址中的博客不在当前图谱中。<button
            type="button"
            class="min-h-11 font-medium underline"
            onclick={explorer.clearInvalid}>清除无效选择</button
          >
        </div>{/if}
      <div
        class={[
          'relative min-h-0 min-w-0',
          fullscreen ? 'flex flex-1 flex-col md:flex-row' : 'grid',
          panelOpen && !fullscreen && 'md:grid-cols-[minmax(0,1fr)_20rem]',
        ]}
      >
        <div
          class={[
            'relative min-h-0 min-w-0 bg-canvas',
            fullscreen ? 'flex-1' : local ? 'h-112 sm:h-140' : 'h-112 sm:h-160',
          ]}
        >
          {#if explorer.visible.size === 0}<div
              class="flex h-full min-h-80 flex-col items-center justify-center gap-3 px-6 text-center text-sm text-fg-muted"
            >
              <IconTopologyStar3 size={32} />
              <p>{graph.nodes.length ? '暂无公开友链，可查看全部博客。' : '暂无公开友链图谱'}</p>
              {#if graph.nodes.length}<button
                  class={buttonClasses}
                  type="button"
                  onclick={() => explorer.setDisplay({ isolated: true, scope: 'all' })}
                  >显示全部博客</button
                >{/if}
            </div>
          {:else if renderFailed || explorer.layoutFailed}<div
              class="flex h-full min-h-80 flex-col items-center justify-center gap-3 px-6 text-center text-sm text-fg-muted"
            >
              <p>图谱视图暂不可用，仍可查看博客和友链列表。</p>
              <button
                type="button"
                class={buttonClasses}
                onclick={() => (explorer.layoutFailed ? void explorer.load() : retryRender())}
                >重试图谱</button
              >
            </div>
          {:else if local}<GraphLocalScene
              {graph}
              selected={explorer.selection.focus}
              visible={explorer.visible}
              {interaction}
              locate={explorer.locate}
              onselect={explorer.select}
              onfocus={locate}
            />
          {:else}{#key renderVersion}<GraphScene
                {graph}
                positions={explorer.positions}
                selected={explorer.selection.focus}
                path={explorer.path ?? []}
                visible={explorer.visible}
                {active}
                {interaction}
                locate={explorer.locate}
                frameRequest={explorer.frameRequest}
                onmode={(next) => (mode = next)}
                onselect={explorer.select}
                onfocus={locate}
                onerror={() => {
                  renderFailed = true;
                  panelOpen = true;
                }}
              />{/key}{/if}
          {#if !local && !renderFailed && !explorer.layoutFailed && explorer.progress < 1}<p
              data-graph-label-obstacle
              class="pointer-events-none absolute top-3 left-3 rounded-sm bg-surface px-3 py-2 text-xs text-fg-muted"
              role="status"
            >
              {explorer.positions.length ? '正在优化布局，可开始浏览' : '正在准备图谱…'}
            </p>{/if}
        </div>
        <aside
          data-graph-label-obstacle
          id={`${uid}-panel`}
          hidden={!panelOpen}
          class={[
            'min-h-0 min-w-0 overflow-y-auto overscroll-contain border-t border-line bg-surface md:w-80 md:shrink-0 md:border-t-0 md:border-l',
            fullscreen
              ? 'absolute inset-x-0 bottom-0 z-20 max-h-[60%] shadow-sm md:static md:max-h-none md:shadow-none'
              : local
                ? 'max-h-100 md:max-h-140'
                : 'max-h-100 md:max-h-160',
          ]}
          aria-label="博客与关系"
        >
          {#if explorer.searching}<div
              class="flex items-center justify-between gap-2 border-b border-line px-4 py-2 text-xs text-fg-muted"
              aria-live="polite"
            >
              <span
                >{explorer.searchPending
                  ? '正在搜索…'
                  : `${explorer.totalResults.toLocaleString()} 个${explorer.query ? '搜索结果' : '博客'}`}</span
              >{#if explorer.selection.focus}<button
                  type="button"
                  class="min-h-11 text-tint-fg"
                  onclick={() => explorer.setSearching(false)}>返回友链</button
                >{/if}
            </div>
            <div data-graph-results>
              {#if explorer.results.length}<GraphNodeList
                  nodes={explorer.results}
                  selected={explorer.selection.focus}
                  onselect={locate}
                  paginated={false}
                />{:else if !explorer.searchPending}<p
                  class="p-6 text-center text-sm text-fg-muted"
                >
                  未找到匹配的博客
                </p>{/if}
            </div>
            {#if explorer.results.length < explorer.totalResults}<button
                type="button"
                disabled={explorer.searchPending}
                class="min-h-11 w-full text-sm font-medium text-tint-fg hover:bg-tint disabled:opacity-50"
                onclick={explorer.more}>加载更多</button
              >{/if}
          {:else}<GraphSidebar
              {graph}
              index={explorer.index}
              selected={explorer.selection.focus}
              {local}
              relation={explorer.relation}
              onrelation={explorer.setRelation}
              onselect={select}
              onlocate={locate}
              onsetfrom={() => {
                explorer.update({ ...explorer.selection, from: explorer.selection.focus });
                pathOpen = true;
              }}
              onsetto={() => {
                explorer.update({ ...explorer.selection, to: explorer.selection.focus });
                pathOpen = true;
              }}
            />{/if}
        </aside>
      </div>
      <div
        class="flex shrink-0 flex-wrap items-center justify-between gap-2 border-t border-line px-3 py-2 text-xs text-fg-muted sm:px-4"
      >
        <div class="flex min-w-0 items-center gap-2">
          {#if coarse && !fullscreen}<button
              type="button"
              class={[buttonClasses, touchActive && 'bg-tint text-tint-fg']}
              aria-pressed={touchActive}
              onclick={() => (touchActive = !touchActive)}
              ><IconHandMove size={16} />{touchActive ? '返回页面滚动' : '操作图谱'}</button
            >{/if}
          {#if selectedNode && !panelOpen}<button
              type="button"
              class="min-h-10 max-w-48 truncate font-medium text-tint-fg"
              onclick={() => (panelOpen = true)}>{selectedNode.name}</button
            >{/if}
          <details class="relative">
            <summary class="cursor-pointer py-2">操作说明</summary>
            <div
              class="absolute bottom-full left-0 z-20 mb-2 w-64 rounded-md border border-line bg-surface p-4 text-xs/relaxed shadow-sm"
            >
              {#if coarse}<p>
                  {fullscreen || touchActive
                    ? local
                      ? '单指平移，双指缩放或平移，双击博客聚焦其直接友链。'
                      : '单指按当前模式平移或旋转，双指缩放或平移，双击博客聚焦其直接友链。'
                    : '单指滑动页面，轻点选择博客；开启“操作图谱”后可拖动和缩放。'}
                </p>
              {:else}<p>
                  {local
                    ? '拖动平移。'
                    : '左键按模式平移或旋转，右键平移。'}滚轮缩放，双击博客聚焦其直接友链。
                </p>
                <p class="mt-2">图谱获得焦点后，方向键平移，加减键缩放，Home 适配视图。</p>{/if}
            </div>
          </details>
        </div>
        {#if !local}<button
            type="button"
            class="inline-flex min-h-10 items-center gap-2 font-medium text-tint-fg"
            aria-expanded={pathOpen}
            onclick={() => (pathOpen = !pathOpen)}
            >最短路径{#if pathOpen}<IconChevronDown size={16} />{:else}<IconChevronUp
                size={16}
              />{/if}</button
          >{/if}
      </div>
      {#if !local && pathOpen}<div
          class={['shrink-0 overflow-y-auto', fullscreen && 'max-h-[35dvh]']}
        >
          <GraphPath
            nodes={graph.nodes}
            selection={explorer.selection}
            path={explorer.path}
            onchange={explorer.update}
            onselect={locate}
          />
        </div>{/if}
    </div>
  {/if}
</section>

import { requestSiteGraph } from '../../api/sites/site-graph.browser.ts';
import type { GraphNode, SiteGraph } from '../../api/sites/site-graph.types.ts';

import {
  type GraphDisplay,
  type GraphRelation,
  indexGraph,
  visibleGraphNodes,
} from './site-graph.model.ts';
import type {
  GraphLayoutRequest,
  GraphLayoutResponse,
  GraphWorkerRequest,
  GraphWorkerResponse,
} from './site-graph.protocol.ts';
import {
  type GraphSelection,
  graphViewHref,
  readGraphDisplay,
  readGraphSelection,
} from './site-graph.shared.ts';
import { createGraphLayoutWorker, createGraphQueryWorker } from './site-graph.workers.browser.ts';

export function createGraphExplorer(identifier: () => string | undefined, active: () => boolean) {
  let graph = $state.raw<SiteGraph>();
  let loading = $state(true);
  let failed = $state(false);
  let layoutFailed = $state(false);
  let positions = $state.raw<Float32Array>(new Float32Array());
  let progress = $state(0);
  let query = $state('');
  let searching = $state(true);
  let searchPending = $state(true);
  let results = $state.raw<readonly GraphNode[]>([]);
  let totalResults = $state(0);
  let limit = 40;
  let selection = $state<GraphSelection>({ focus: '', from: '', to: '', direction: 'directed' });
  let display = $state<GraphDisplay>({ isolated: false, scope: 'all' });
  let relation = $state<GraphRelation>('all');
  let path = $state.raw<readonly string[] | null>();
  let invalid = $state(false);
  let locate = $state({ id: '', request: 0 });
  let frameRequest = $state(0);
  let queryWorker: Worker | undefined;
  let layoutWorker = $state.raw<Worker>();
  let controller: AbortController | undefined;
  let disposed = false;
  let searchRequest = 0;
  let pathRequest = 0;
  let version = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const local = $derived(Boolean(identifier()));
  const index = $derived(graph ? indexGraph(graph) : undefined);
  const visible = $derived(
    index
      ? visibleGraphNodes(index, {
          ...display,
          focus: selection.focus,
          path: path ?? [],
          ...(local && graph?.centerId ? { center: graph.centerId, relation } : {}),
        })
      : new SvelteSet<string>(),
  );
  function send(message: GraphWorkerRequest): void {
    queryWorker?.postMessage(message);
  }
  function layout(message: GraphLayoutRequest): void {
    layoutWorker?.postMessage(message);
  }
  function syncActive(): void {
    layout({ type: active() && !document.hidden ? 'resume' : 'pause', version });
  }
  $effect(() => {
    if (layoutWorker) syncActive();
  });
  function history(): void {
    if (!local) window.history.pushState(null, '', graphViewHref(selection, display));
  }
  function update(next: GraphSelection, record = true): void {
    invalid = !local && [next.focus, next.from, next.to].some((id) => id && !index?.nodes.has(id));
    const previous = selection;
    selection = {
      ...next,
      focus: index?.nodes.has(next.focus) ? next.focus : local ? (graph?.centerId ?? '') : '',
      from: index?.nodes.has(next.from) ? next.from : '',
      to: index?.nodes.has(next.to) ? next.to : '',
    };
    if (selection.focus && !index?.connected.has(selection.focus))
      display = { ...display, isolated: true };
    if (!selection.focus) display = { ...display, scope: 'all' };
    if (record) history();
    if (
      previous.from !== selection.from ||
      previous.to !== selection.to ||
      previous.direction !== selection.direction
    ) {
      path = undefined;
      pathRequest++;
      if (selection.from && selection.to)
        send({ type: 'path', ...selection, requestId: pathRequest });
    }
  }
  function select(id: string): void {
    if (local && !visible.has(id)) relation = 'all';
    update({ ...selection, focus: id });
    searching = false;
  }
  function focus(id: string): void {
    select(id);
    locate = { id, request: locate.request + 1 };
  }
  function requestSearch(): void {
    send({ type: 'search', query, requestId: searchRequest, limit });
  }
  function search(value: string, immediate = false): void {
    clearTimeout(timer);
    query = value;
    searching = true;
    searchPending = true;
    limit = 40;
    searchRequest++;
    if (immediate) requestSearch();
    else timer = setTimeout(requestSearch, 120);
  }
  function more(): void {
    limit += 40;
    searchPending = true;
    searchRequest++;
    requestSearch();
  }
  async function load(): Promise<void> {
    clearTimeout(timer);
    controller?.abort();
    queryWorker?.terminate();
    layoutWorker?.terminate();
    controller = new AbortController();
    const signal = controller.signal;
    const current = ++version;
    graph = undefined;
    loading = true;
    failed = false;
    layoutFailed = false;
    positions = new Float32Array();
    progress = 0;
    results = [];
    searchPending = true;
    try {
      const data = await requestSiteGraph(identifier(), signal);
      if (disposed || signal.aborted) return;
      graph = data;
      queryWorker = createGraphQueryWorker();
      queryWorker.onmessage = (event: MessageEvent<GraphWorkerResponse>) => {
        if (current !== version) return;
        const message = event.data;
        switch (message.type) {
          case 'search':
            if (message.requestId !== searchRequest) return;
            results = message.ids.flatMap((id) => {
              const item = index?.nodes.get(id);
              return item ? [item.node] : [];
            });
            totalResults = message.total;
            searchPending = false;
            break;
          case 'path':
            if (message.requestId === pathRequest) path = message.ids;
            break;
          case 'error':
            failed = true;
            queryWorker?.terminate();
            break;
        }
      };
      queryWorker.onerror = () => {
        if (current === version) failed = true;
      };
      send({ type: 'init', graph: data });
      if (!local) {
        layoutWorker = createGraphLayoutWorker();
        layoutWorker.onmessage = (event: MessageEvent<GraphLayoutResponse>) => {
          const message = event.data;
          if (message.version !== version) return;
          if (message.type === 'layout') {
            if (
              !positions.length ||
              message.progress === 1 ||
              !window.matchMedia('(prefers-reduced-motion: reduce)').matches
            )
              positions = message.positions;
            progress = message.progress;
          } else {
            layoutFailed = true;
            layoutWorker?.terminate();
          }
        };
        layoutWorker.onerror = () => {
          if (current === version) layoutFailed = true;
        };
        layout({ type: 'init', graph: data, version });
        syncActive();
        display = readGraphDisplay(new SvelteURLSearchParams(window.location.search));
      } else progress = 1;
      selection = { focus: '', from: '', to: '', direction: 'directed' };
      update(
        local
          ? { ...selection, focus: data.centerId ?? '' }
          : readGraphSelection(new SvelteURLSearchParams(window.location.search)),
        false,
      );
      locate = { id: local ? '' : selection.focus, request: locate.request + 1 };
      search('', true);
      searching = !selection.focus;
    } catch {
      if (!disposed && !signal.aborted) failed = true;
    } finally {
      if (!disposed && !signal.aborted) loading = false;
    }
  }
  const popstate = () => {
    display = readGraphDisplay(new SvelteURLSearchParams(window.location.search));
    update(readGraphSelection(new SvelteURLSearchParams(window.location.search)), false);
    locate = { id: selection.focus, request: locate.request + 1 };
  };
  return {
    get graph() {
      return graph;
    },
    get index() {
      return index;
    },
    get loading() {
      return loading;
    },
    get failed() {
      return failed;
    },
    get layoutFailed() {
      return layoutFailed;
    },
    get positions() {
      return positions;
    },
    get progress() {
      return progress;
    },
    get query() {
      return query;
    },
    get searching() {
      return searching;
    },
    get searchPending() {
      return searchPending;
    },
    get results() {
      return results;
    },
    get totalResults() {
      return totalResults;
    },
    get selection() {
      return selection;
    },
    get display() {
      return display;
    },
    get path() {
      return path;
    },
    get visible() {
      return visible;
    },
    get locate() {
      return locate;
    },
    get frameRequest() {
      return frameRequest;
    },
    get relation() {
      return relation;
    },
    get invalid() {
      return invalid;
    },
    load,
    update,
    select,
    focus,
    search,
    more,
    clearInvalid(): void {
      invalid = false;
      history();
    },
    setSearching(value: boolean): void {
      searching = value;
    },
    setRelation(value: GraphRelation): void {
      relation = value;
      if (local && !visible.has(selection.focus) && graph?.centerId) select(graph.centerId);
    },
    setDisplay(value: GraphDisplay): void {
      display = value;
      frameRequest++;
      if (!value.isolated && selection.focus && !index?.connected.has(selection.focus)) {
        selection = { ...selection, focus: '' };
        display = { ...display, scope: 'all' };
        searching = true;
      }
      history();
    },
    mount(): () => void {
      disposed = false;
      void load();
      if (!local) window.addEventListener('popstate', popstate);
      document.addEventListener('visibilitychange', syncActive);
      return () => {
        disposed = true;
        version++;
        clearTimeout(timer);
        controller?.abort();
        queryWorker?.terminate();
        layoutWorker?.terminate();
        window.removeEventListener('popstate', popstate);
        document.removeEventListener('visibilitychange', syncActive);
      };
    },
  };
}
import { SvelteSet, SvelteURLSearchParams } from 'svelte/reactivity';

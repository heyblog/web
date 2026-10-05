import { createGraphSearch, createPathFinder } from './site-graph.engine.ts';
import type { GraphWorkerRequest, GraphWorkerResponse } from './site-graph.protocol.ts';

let findPath: ReturnType<typeof createPathFinder> | undefined;
let search: ReturnType<typeof createGraphSearch> | undefined;

function respond(message: GraphWorkerResponse): void {
  self.postMessage(message);
}

self.onmessage = (event: MessageEvent<GraphWorkerRequest>) => {
  const message = event.data;
  try {
    switch (message.type) {
      case 'init':
        findPath = createPathFinder(message.graph);
        search = createGraphSearch(message.graph);
        break;
      case 'search':
        if (search)
          respond({
            type: 'search',
            ...search(message.query, message.limit),
            requestId: message.requestId,
          });
        break;
      case 'path':
        if (findPath)
          respond({
            type: 'path',
            ids: findPath(message.from, message.to, message.direction),
            requestId: message.requestId,
          });
        break;
      default:
        message satisfies never;
    }
  } catch {
    // no-excuse-ok: catch -- the worker boundary reports a sanitized failure.
    respond({ type: 'error' });
  }
};

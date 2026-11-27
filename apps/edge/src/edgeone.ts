import { createApp } from './app.ts';
import { edgeOneFetch } from './edgeone-fetch.ts';

// EdgeOne exposes encrypted function variables on the runtime's env object.
declare const env: { readonly EDGE_SERVICE_TOKEN?: unknown } | undefined;
type EdgeEvent = {
  readonly request: Request;
  respondWith(response: Promise<Response> | Response): void;
};
declare function addEventListener(type: 'fetch', handler: (event: EdgeEvent) => void): void;
addEventListener('fetch', (event) => {
  const request = event.request;
  const token = typeof env === 'undefined' ? undefined : env?.EDGE_SERVICE_TOKEN;
  event.respondWith(
    createApp({
      platform: 'EO',
      token: typeof token === 'string' ? token : '',
      // Request.eo.geo describes the caller, not the serving node. No documented
      // serving-node identifier is currently available in this runtime contract.
      node: {},
      fetch: edgeOneFetch(request.url, fetch),
    }).fetch(request),
  );
});

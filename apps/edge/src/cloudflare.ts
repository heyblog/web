import { createApp } from './app.ts';

type Env = { readonly EDGE_SERVICE_TOKEN: string };
type RequestMetadata = Request & {
  readonly cf?: { readonly colo?: string; readonly country?: string };
};
// Cloudflare requires a default module-worker export. No private network bindings are used.
export default {
  fetch(request: RequestMetadata, env: Env): Promise<Response> | Response {
    return createApp({
      platform: 'CF',
      token: env.EDGE_SERVICE_TOKEN,
      node: { colo: request.cf?.colo ?? '' },
      // Native fetch is required for platform egress policy and manual redirect semantics.
      fetch: (target, init) => fetch(target.url, { ...init, cache: 'no-store' }),
    }).fetch(request);
  },
};

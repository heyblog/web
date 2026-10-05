import type { SiteGraph } from '../../api/sites/site-graph.types.ts';

export type GraphWorkerRequest =
  | { readonly type: 'init'; readonly graph: SiteGraph }
  | {
      readonly type: 'search';
      readonly query: string;
      readonly requestId: number;
      readonly limit: number;
    }
  | {
      readonly type: 'path';
      readonly from: string;
      readonly to: string;
      readonly direction: 'directed' | 'undirected';
      readonly requestId: number;
    };

export type GraphWorkerResponse =
  | {
      readonly type: 'search';
      readonly ids: readonly string[];
      readonly total: number;
      readonly requestId: number;
    }
  | { readonly type: 'path'; readonly ids: readonly string[] | null; readonly requestId: number }
  | { readonly type: 'error' };

export type GraphLayoutRequest =
  | { readonly type: 'init'; readonly graph: SiteGraph; readonly version: number }
  | { readonly type: 'pause' | 'resume' | 'cancel'; readonly version: number };

export type GraphLayoutResponse =
  | {
      readonly type: 'layout';
      readonly positions: Float32Array<ArrayBuffer>;
      readonly progress: number;
      readonly version: number;
    }
  | { readonly type: 'error'; readonly version: number };

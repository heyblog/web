import {
  type ApiJsonDependencies,
  type ApiJsonResult,
  fetchApiJson,
} from '../transport/client.server.ts';

import type { SiteCardView } from './site-card.types.ts';

export interface RandomSiteView {
  site: SiteCardView | null;
}

export function loadRandomSite(
  search: string,
  request?: Request,
  dependencies: ApiJsonDependencies = {},
): Promise<ApiJsonResult<RandomSiteView>> {
  return fetchApiJson<RandomSiteView>(`/sites/random${search}`, {
    ...dependencies,
    request,
    signal: request?.signal,
  });
}

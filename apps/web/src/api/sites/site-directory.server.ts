import {
  type ApiJsonDependencies,
  type ApiJsonResult,
  fetchApiJson,
} from '../transport/client.server.ts';

import { buildSiteDirectorySearchParams } from './site-directory.params.ts';
import type {
  SiteDirectoryOptions,
  SiteDirectoryQuery,
  SiteDirectoryView,
} from './site-directory.types.ts';

export function loadSiteDirectory(
  query: SiteDirectoryQuery,
  request?: Request,
): Promise<ApiJsonResult<SiteDirectoryView>> {
  const parameters = buildSiteDirectorySearchParams(query);
  return fetchApiJson<SiteDirectoryView>(`/sites?${parameters.toString()}`, {
    request,
    signal: request?.signal,
  });
}

export function loadSiteDirectoryOptions(
  request?: Request,
  dependencies: ApiJsonDependencies = {},
): Promise<ApiJsonResult<SiteDirectoryOptions>> {
  return fetchApiJson<SiteDirectoryOptions>('/sites/options', {
    ...dependencies,
    request,
    signal: request?.signal,
  });
}

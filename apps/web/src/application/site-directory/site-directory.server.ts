import { type ApiJsonResult, fetchApiJson } from '@/application/api/client.server';

import type {
  SiteDirectoryOptions,
  SiteDirectoryQuery,
  SiteDirectoryView,
} from './site-directory.models';
import { buildSiteDirectorySearchParams } from './site-directory.shared';

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
): Promise<ApiJsonResult<SiteDirectoryOptions>> {
  return fetchApiJson<SiteDirectoryOptions>('/sites/options', { request, signal: request?.signal });
}

import { type ApiJsonResult, fetchApiJson } from '@/application/api/client.server';

import type {
  SiteDirectoryOptions,
  SiteDirectoryQuery,
  SiteDirectoryView,
} from './site-directory.models';
import { buildSiteDirectorySearchParams } from './site-directory.shared';

export function loadSiteDirectory(
  query: SiteDirectoryQuery,
  signal?: AbortSignal,
): Promise<ApiJsonResult<SiteDirectoryView>> {
  const parameters = buildSiteDirectorySearchParams(query);
  return fetchApiJson<SiteDirectoryView>(`/sites?${parameters.toString()}`, { signal });
}

export function loadSiteDirectoryOptions(
  signal?: AbortSignal,
): Promise<ApiJsonResult<SiteDirectoryOptions>> {
  return fetchApiJson<SiteDirectoryOptions>('/sites/options', { signal });
}

import type { SiteDirectoryQuery, SiteDirectoryView } from './site-directory.models';
import { buildSiteDirectorySearchParams } from './site-directory.shared';

export class SiteDirectoryRequestError extends Error {
  constructor(readonly status: number) {
    super('Site directory request failed');
    this.name = 'SiteDirectoryRequestError';
  }
}

export async function refreshSiteDirectory(
  query: SiteDirectoryQuery,
  signal: AbortSignal,
): Promise<SiteDirectoryView> {
  const parameters = buildSiteDirectorySearchParams(query);
  const response = await fetch(`/api/site-directory?${parameters.toString()}`, {
    headers: { Accept: 'application/json' },
    cache: 'no-store',
    signal,
  });
  if (!response.ok) throw new SiteDirectoryRequestError(response.status);
  return (await response.json()) as SiteDirectoryView;
}

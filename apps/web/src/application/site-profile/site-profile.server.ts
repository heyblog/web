import { type ApiJsonResult, fetchApiJson } from '../api/client.server.ts';
import { type SiteCardBase, siteDetailPath } from '../home/home.shared.ts';

export interface SiteTopic {
  name: string;
  slug: string;
  description: string;
  role: 'PRIMARY' | 'SECONDARY';
}

export interface SiteWarning {
  name: string;
  slug: string;
  description: string;
}

export interface SiteFeed {
  name: string;
  url: string;
  format: 'UNKNOWN' | 'RSS' | 'ATOM' | 'JSON';
  isDefault: boolean;
}

export interface SiteResource {
  kind: 'SITEMAP' | 'LINK_PAGE';
  url: string;
}

export interface SiteTechnology {
  name: string;
  role: 'SITE_PROGRAM' | 'FRAMEWORK' | 'LANGUAGE' | 'RUNTIME' | 'OTHER';
  homepageUrl: string | null;
  repositoryUrl: string | null;
  isOpenSource: boolean;
}

export interface SiteProfile extends SiteCardBase {
  topics: SiteTopic[];
  warnings: SiteWarning[];
  feeds: SiteFeed[];
  resources: SiteResource[];
  technologies: SiteTechnology[];
}

const shortIdPattern = /^[0-9A-Za-z]{9}$/;
const uuidPattern = /^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$/;
const customIdPattern = /^[0-9A-Za-z](?!.*[_-]{2})[0-9A-Za-z_-]{1,30}[0-9A-Za-z]$/;

function assertNever(value: never): never {
  throw new TypeError(`Unexpected site profile result: ${JSON.stringify(value)}`);
}

export function loadSiteByIdentifier(
  identifier: string,
  signal?: AbortSignal,
): Promise<ApiJsonResult<SiteProfile>> {
  if (!shortIdPattern.test(identifier) && !uuidPattern.test(identifier)) {
    return Promise.resolve({ kind: 'not-found' });
  }
  return fetchApiJson<SiteProfile>(`/sites/id/${encodeURIComponent(identifier)}`, { signal });
}

export function loadSiteByCustomID(
  customID: string,
  signal?: AbortSignal,
): Promise<ApiJsonResult<SiteProfile>> {
  if (!customIdPattern.test(customID)) {
    return Promise.resolve({ kind: 'not-found' });
  }
  return fetchApiJson<SiteProfile>(`/sites/custom/${encodeURIComponent(customID)}`, { signal });
}

export function canonicalSitePath(profile: SiteProfile): string {
  return siteDetailPath(profile);
}

export function canonicalSiteRedirectPath(
  identifier: string,
  result: ApiJsonResult<SiteProfile>,
): string | null {
  switch (result.kind) {
    case 'success':
      return identifier === result.data.shortId ? null : canonicalSitePath(result.data);
    case 'bad-request':
    case 'not-found':
    case 'unavailable':
      return null;
    default:
      return assertNever(result);
  }
}

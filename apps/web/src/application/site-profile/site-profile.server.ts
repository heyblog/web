import { type ApiJsonResult, fetchApiJson } from '@/application/api/client.server';
import { type SiteCardBase, siteDetailPath } from '@/application/home/home.shared';

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

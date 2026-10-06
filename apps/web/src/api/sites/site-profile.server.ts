import { type ApiJsonResult, fetchApiJson } from '../transport/client.server.ts';

import { type SiteProfile } from './site-profile.types.ts';

const shortIdPattern = /^[0-9A-Za-z]{9}$/;
const uuidPattern = /^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$/;
const customIdPattern = /^[0-9A-Za-z](?!.*[_-]{2})[0-9A-Za-z_-]{1,30}[0-9A-Za-z]$/;

export function loadSiteByIdentifier(
  identifier: string,
  request?: Request,
): Promise<ApiJsonResult<SiteProfile>> {
  if (!shortIdPattern.test(identifier) && !uuidPattern.test(identifier)) {
    return Promise.resolve({ kind: 'not-found' });
  }
  return fetchApiJson<SiteProfile>(`/sites/id/${encodeURIComponent(identifier)}`, {
    request,
    signal: request?.signal,
  });
}

export function loadSiteMetadataByIdentifier(
  identifier: string,
  request?: Request,
): Promise<ApiJsonResult<SiteProfile>> {
  if (!shortIdPattern.test(identifier) && !uuidPattern.test(identifier))
    return Promise.resolve({ kind: 'not-found' });
  return fetchApiJson<SiteProfile>(`/sites/id/${encodeURIComponent(identifier)}/metadata`, {
    request,
    signal: request?.signal,
  });
}

export function loadSiteByCustomID(
  customID: string,
  request?: Request,
): Promise<ApiJsonResult<SiteProfile>> {
  if (!customIdPattern.test(customID)) {
    return Promise.resolve({ kind: 'not-found' });
  }
  return fetchApiJson<SiteProfile>(`/sites/custom/${encodeURIComponent(customID)}`, {
    request,
    signal: request?.signal,
  });
}

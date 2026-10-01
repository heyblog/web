import { type SiteProfile } from '../../api/sites/site-profile.types.ts';
import { type ApiJsonResult } from '../../api/transport/client.server.ts';

import { siteDetailPath } from './site-card.shared.ts';

export function canonicalSitePath(profile: SiteProfile): string {
  return siteDetailPath(profile);
}

function assertNever(value: never): never {
  throw new TypeError(`Unexpected site profile result: ${JSON.stringify(value)}`);
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

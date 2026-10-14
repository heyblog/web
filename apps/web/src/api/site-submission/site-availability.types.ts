import { parseSiteSearchResult } from './site-search.types.ts';
import type { SiteAvailability } from './site-submission.types.ts';

export function parseSiteAvailability(value: unknown): SiteAvailability {
  if (
    typeof value !== 'object' ||
    value === null ||
    !('available' in value) ||
    typeof value.available !== 'boolean'
  )
    throw new TypeError('Invalid site availability');
  if (value.available) return { available: true };
  if (!('existing_site' in value)) throw new TypeError('Missing conflicting site');
  const existing = parseSiteSearchResult(value.existing_site);
  if (!existing.short_id) throw new TypeError('Missing conflicting site identity');
  return { available: false, existing_site: existing };
}

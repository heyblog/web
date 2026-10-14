import type { SiteSearchResult } from './site-submission.types.ts';

export function parseSiteSearch(value: unknown): readonly SiteSearchResult[] {
  if (
    typeof value !== 'object' ||
    value === null ||
    !('items' in value) ||
    !Array.isArray(value.items)
  )
    throw new TypeError('Invalid site search');
  return value.items.map(parseSiteSearchResult);
}

export function parseSiteSearchResult(item: unknown): SiteSearchResult {
  if (
    typeof item !== 'object' ||
    item === null ||
    !('short_id' in item) ||
    !('name' in item) ||
    !('url' in item) ||
    !('visibility' in item) ||
    typeof item.short_id !== 'string' ||
    typeof item.name !== 'string' ||
    typeof item.url !== 'string' ||
    typeof item.visibility !== 'string'
  )
    throw new TypeError('Invalid site search item');
  return { short_id: item.short_id, name: item.name, url: item.url, visibility: item.visibility };
}

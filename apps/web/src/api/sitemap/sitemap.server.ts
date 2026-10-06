import { isDate, isRecord, isUUID } from '../announcements/announcements.types.ts';
import { type ApiJsonDependencies, fetchApiJson } from '../transport/client.server.ts';

export type DynamicSitemapKind = 'sites' | 'announcements';
interface SitemapSite {
  readonly id: string;
  readonly shortId: string;
}
interface SitemapAnnouncement {
  readonly id: string;
  readonly startsAt: string;
  readonly publishedAt: string;
  readonly updatedAt: string;
}
export type SitemapItem = SitemapSite | SitemapAnnouncement;
interface SitemapPage {
  readonly items: readonly SitemapItem[];
  readonly nextAfter: string | null;
}

export class SitemapUnavailableError extends Error {
  constructor() {
    super('Public sitemap data is unavailable');
    this.name = 'SitemapUnavailableError';
  }
}

function parsePage(value: unknown, kind: DynamicSitemapKind): SitemapPage | null {
  if (
    !isRecord(value) ||
    !Array.isArray(value.items) ||
    value.items.length > 1_000 ||
    (value.nextAfter !== null && !isUUID(value.nextAfter))
  )
    return null;
  const items: SitemapItem[] = [];
  for (const item of value.items) {
    if (!isRecord(item) || !isUUID(item.id)) return null;
    if (kind === 'sites') {
      if (typeof item.shortId !== 'string' || !/^[0-9A-Za-z]{9}$/.test(item.shortId)) return null;
      items.push({ id: item.id, shortId: item.shortId });
    } else {
      if (!isDate(item.startsAt) || !isDate(item.publishedAt) || !isDate(item.updatedAt))
        return null;
      items.push({
        id: item.id,
        startsAt: item.startsAt,
        publishedAt: item.publishedAt,
        updatedAt: item.updatedAt,
      });
    }
  }
  return { items, nextAfter: value.nextAfter };
}

export async function loadSitemapItems(
  kind: DynamicSitemapKind,
  signal: AbortSignal,
  dependencies: ApiJsonDependencies = {},
): Promise<readonly SitemapItem[]> {
  const items: SitemapItem[] = [];
  let after: string | null = null;
  let previous = '';
  do {
    const parameters = new URLSearchParams({ kind });
    if (after) parameters.set('after', after);
    const result = await fetchApiJson<unknown>(`/sitemap?${parameters.toString()}`, {
      ...dependencies,
      signal,
    });
    const page = result.kind === 'success' ? parsePage(result.data, kind) : null;
    if (!page) throw new SitemapUnavailableError();
    for (const item of page.items) {
      if (item.id.toLowerCase() <= previous) throw new SitemapUnavailableError();
      previous = item.id.toLowerCase();
      items.push(item);
    }
    if (
      page.nextAfter !== null &&
      (page.items.length !== 1_000 || page.nextAfter.toLowerCase() !== previous)
    )
      throw new SitemapUnavailableError();
    after = page.nextAfter;
    signal.throwIfAborted();
  } while (after !== null);
  return items;
}

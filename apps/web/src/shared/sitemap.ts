import { siteConfig } from '../site.config.ts';

import { isIndexablePath } from './indexing.ts';

export interface SitemapEntry {
  readonly path: string;
  readonly lastmod?: string;
}
export type SitemapKind = 'static' | 'sites' | 'announcements';
export const sitemapLimits = { urls: 10_000, bytes: 50 * 1024 * 1024, documents: 50_000 } as const;
const namespace = 'http://www.sitemaps.org/schemas/sitemap/0.9';
const encoder = new TextEncoder();
const utf8Size = (value: string) => encoder.encode(value).byteLength;
const declaration = '<?xml version="1.0" encoding="UTF-8"?>';

export function escapeXml(value: string): string {
  return value.replace(/[<>&"']/g, (character) => {
    switch (character) {
      case '<':
        return '&lt;';
      case '>':
        return '&gt;';
      case '&':
        return '&amp;';
      case '"':
        return '&quot;';
      case "'":
        return '&apos;';
      default:
        return character;
    }
  });
}

export function sitemapUrl(entry: SitemapEntry): string {
  const url = new URL(entry.path, siteConfig.url);
  if (
    url.origin !== new URL(siteConfig.url).origin ||
    url.search ||
    url.hash ||
    !isIndexablePath(url.pathname)
  ) {
    throw new TypeError('Sitemap entry must be an indexable canonical site path');
  }
  return url.href;
}

export function createSitemapDocuments(
  groups: Readonly<Record<SitemapKind, readonly SitemapEntry[]>>,
) {
  const documents = new Map<string, string>();
  for (const kind of ['static', 'sites', 'announcements'] as const) {
    const open = `${declaration}\n<urlset xmlns="${namespace}">\n`;
    const close = '</urlset>\n';
    let nodes: string[] = [];
    let bytes = utf8Size(open + close);
    let part = 1;
    const flush = () => {
      documents.set(`/sitemap-${kind}-${part++}.xml`, open + nodes.join('') + close);
      nodes = [];
      bytes = utf8Size(open + close);
    };
    const seen = new Set<string>();
    for (const entry of groups[kind]) {
      const url = sitemapUrl(entry);
      if (seen.has(url)) continue;
      seen.add(url);
      const date = entry.lastmod;
      if (date !== undefined && !Number.isFinite(Date.parse(date)))
        throw new TypeError('Invalid sitemap modification date');
      const node = `<url><loc>${escapeXml(url)}</loc>${date ? `<lastmod>${escapeXml(date)}</lastmod>` : ''}</url>\n`;
      const size = utf8Size(node);
      if (size + utf8Size(open + close) > sitemapLimits.bytes)
        throw new RangeError('Sitemap entry exceeds size limit');
      if (nodes.length >= sitemapLimits.urls || bytes + size > sitemapLimits.bytes) flush();
      nodes.push(node);
      bytes += size;
    }
    flush();
  }
  if (documents.size > sitemapLimits.documents)
    throw new RangeError('Sitemap index exceeds document limit');
  const index = `${declaration}\n<sitemapindex xmlns="${namespace}">\n${[...documents.keys()].map((path) => `<sitemap><loc>${escapeXml(new URL(path, siteConfig.url).href)}</loc></sitemap>\n`).join('')}</sitemapindex>\n`;
  if (utf8Size(index) > sitemapLimits.bytes)
    throw new RangeError('Sitemap index exceeds size limit');
  documents.set('/sitemap.xml', index);
  return documents;
}

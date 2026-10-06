import {
  readBlogEntries,
  readDocPageEntries,
  readDocsLandingEntry,
  readPageEntries,
} from '../../integrations/content/content.server.ts';
import { isIndexablePath } from '../../shared/indexing.ts';
import { toIsoDate } from '../../shared/seo.ts';
import type { SitemapEntry } from '../../shared/sitemap.ts';
import { getDocRoutePath } from '../static-content/static-content.models.ts';

export async function loadStaticSitemapEntries(): Promise<readonly SitemapEntry[]> {
  const [blogs, docs, landing, pages] = await Promise.all([
    readBlogEntries(),
    readDocPageEntries(),
    readDocsLandingEntry(),
    readPageEntries(),
  ]);
  const entries: SitemapEntry[] = [
    '/',
    '/site',
    '/graph',
    '/blog',
    '/announcements',
    '/members',
  ].map((path) => ({ path }));
  entries.push({ path: '/docs', lastmod: toIsoDate(landing.data.update_time) });
  for (const entry of blogs)
    entries.push({ path: `/blog/${entry.id}`, lastmod: toIsoDate(entry.data.update_time) });
  for (const entry of docs)
    entries.push({ path: getDocRoutePath(entry.id), lastmod: toIsoDate(entry.data.update_time) });
  for (const entry of pages)
    if (isIndexablePath(`/${entry.id}`))
      entries.push({ path: `/${entry.id}`, lastmod: toIsoDate(entry.data.update_time) });
  return entries;
}

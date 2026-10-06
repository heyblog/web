import { loadSitemapItems } from '../../api/sitemap/sitemap.server.ts';

import { createSitemapService } from './sitemap-cache.server.ts';
import { loadStaticSitemapEntries } from './static-sitemap.server.ts';

export const serveSitemap = createSitemapService({
  loadStatic: loadStaticSitemapEntries,
  loadDynamic: loadSitemapItems,
});

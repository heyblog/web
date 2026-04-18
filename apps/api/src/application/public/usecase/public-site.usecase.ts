import {
  Programs,
  SiteAccessCounters,
  SiteArchitectures,
  SiteFeedArticleStats,
  Sites,
  SiteTags,
  TagDefinitions,
} from '@zhblogs/db';

import { and, asc, desc, eq, inArray } from 'drizzle-orm';
import type { FastifyInstance } from 'fastify';

import {
  compareNames,
  createSiteSlug,
  matchesSiteSlug,
} from '@/application/public/usecase/public-site.directory.core';
import {
  collectSiteTags,
  matchesDirectoryFilters,
  sortDirectoryItems,
} from '@/application/public/usecase/public-site.directory-filter';
import { normalizeDirectoryQuery } from '@/application/public/usecase/public-site.directory-query';
import type {
  PublicSiteBaseRow,
  PublicSiteDirectoryItem,
  PublicSiteDirectoryMeta,
  PublicSiteDirectoryQuery,
  PublicSiteDirectoryResult,
  PublicSiteWarningTag,
  SiteArchitectureFilterState,
} from '@/application/public/usecase/public-site.types';
import { listSiteWarningTagsBySiteIds } from '@/application/sites/usecase/site-warning-tag.usecase';

export type {
  PublicSiteDirectoryItem,
  PublicSiteDirectoryMeta,
  PublicSiteDirectoryQuery,
  PublicSiteDirectoryResult,
  PublicSiteWarningTag,
} from '@/application/public/usecase/public-site.types';

async function loadPublicSiteBaseRows(app: FastifyInstance): Promise<PublicSiteBaseRow[]> {
  return app.db.read
    .select({
      id: Sites.id,
      bid: Sites.bid,
      name: Sites.name,
      url: Sites.url,
      sign: Sites.sign,
      feeds: Sites.feed,
      sitemap: Sites.sitemap,
      linkPage: Sites.link_page,
      featured: Sites.recommend,
      status: Sites.status,
      accessScope: Sites.access_scope,
      joinTime: Sites.join_time,
      updateTime: Sites.update_time,
      reason: Sites.reason,
    })
    .from(Sites)
    .where(eq(Sites.is_show, true))
    .orderBy(desc(Sites.recommend), desc(Sites.update_time), desc(Sites.join_time));
}

async function enrichPublicSites(
  app: FastifyInstance,
  siteRows: PublicSiteBaseRow[],
): Promise<PublicSiteDirectoryItem[]> {
  if (siteRows.length === 0) {
    return [];
  }

  const siteIds = siteRows.map((site) => site.id);
  const [statsRows, accessRows, tagRows, warningRows] = await Promise.all([
    app.db.read
      .select({
        site_id: SiteFeedArticleStats.site_id,
        visible_articles: SiteFeedArticleStats.visible_articles,
        total_articles: SiteFeedArticleStats.total_articles,
        latest_published_time: SiteFeedArticleStats.latest_published_time,
      })
      .from(SiteFeedArticleStats)
      .where(inArray(SiteFeedArticleStats.site_id, siteIds)),
    app.db.read
      .select({
        site_id: SiteAccessCounters.site_id,
        total: SiteAccessCounters.total,
      })
      .from(SiteAccessCounters)
      .where(inArray(SiteAccessCounters.site_id, siteIds)),
    app.db.read
      .select({
        site_id: SiteTags.site_id,
        tagName: TagDefinitions.name,
        tagType: TagDefinitions.tag_type,
      })
      .from(SiteTags)
      .innerJoin(TagDefinitions, eq(SiteTags.tag_id, TagDefinitions.id))
      .where(and(inArray(SiteTags.site_id, siteIds), eq(TagDefinitions.is_enabled, true))),
    listSiteWarningTagsBySiteIds(app, siteIds),
  ]);

  const statsBySiteId = createDirectoryStatsMap(statsRows);
  const accessCountBySiteId = new Map(
    accessRows.flatMap((row) => (row.site_id ? [[row.site_id, row.total ?? 0] as const] : [])),
  );
  const tagStateBySiteId = collectSiteTags(tagRows);
  const warningTagsBySiteId = createWarningTagMap(warningRows);

  return siteRows.map((site) =>
    createDirectoryItem(
      site,
      statsBySiteId,
      accessCountBySiteId,
      tagStateBySiteId,
      warningTagsBySiteId,
    ),
  );
}

function createDirectoryStatsMap(
  rows: Array<{
    site_id: string | null;
    visible_articles: number | null;
    total_articles: number | null;
    latest_published_time: Date | null;
  }>,
) {
  return new Map(
    rows.flatMap((row) =>
      row.site_id
        ? [
            [
              row.site_id,
              {
                articleCount: row.visible_articles ?? row.total_articles ?? 0,
                latestPublishedTime: row.latest_published_time,
              },
            ] as const,
          ]
        : [],
    ),
  );
}

function createWarningTagMap(rows: Awaited<ReturnType<typeof listSiteWarningTagsBySiteIds>>) {
  const warningTagsBySiteId = new Map<string, PublicSiteWarningTag[]>();

  for (const row of rows) {
    const current = warningTagsBySiteId.get(row.siteId) ?? [];
    current.push({
      machineKey: row.machineKey,
      name: row.name,
      description: row.description,
    });
    warningTagsBySiteId.set(row.siteId, current);
  }

  return warningTagsBySiteId;
}

function createDirectoryItem(
  site: PublicSiteBaseRow,
  statsBySiteId: Map<
    string,
    {
      articleCount: number;
      latestPublishedTime: Date | null;
    }
  >,
  accessCountBySiteId: Map<string, number>,
  tagStateBySiteId: ReturnType<typeof collectSiteTags>,
  warningTagsBySiteId: Map<string, PublicSiteWarningTag[]>,
): PublicSiteDirectoryItem {
  const stats = statsBySiteId.get(site.id);
  const tags = tagStateBySiteId.get(site.id);

  return {
    id: site.id,
    bid: site.bid,
    slug: createSiteSlug(site),
    name: site.name,
    url: site.url,
    sign: site.sign ?? '',
    feedUrl:
      site.feeds.find((item) => item?.isDefault === true && typeof item.url === 'string')?.url ??
      null,
    sitemap: site.sitemap ?? null,
    linkPage: site.linkPage ?? null,
    featured: site.featured,
    status: site.status,
    accessScope: site.accessScope,
    joinTime: site.joinTime.toISOString(),
    updateTime: site.updateTime.toISOString(),
    latestPublishedTime: stats?.latestPublishedTime?.toISOString() ?? null,
    articleCount: stats?.articleCount ?? 0,
    visitCount: accessCountBySiteId.get(site.id) ?? 0,
    primaryTag: tags?.primaryTag ?? null,
    subTags: tags?.subTags ?? [],
    warningTags: warningTagsBySiteId.get(site.id) ?? [],
  };
}

async function loadArchitectureFilterState(
  app: FastifyInstance,
  siteIds: string[],
): Promise<SiteArchitectureFilterState> {
  if (siteIds.length === 0) {
    return {
      programBySiteId: new Map(),
      filters: {
        programs: [],
      },
    };
  }

  const rows = await app.db.read
    .select({
      siteId: SiteArchitectures.site_id,
      programId: Programs.id,
      programName: Programs.name,
    })
    .from(SiteArchitectures)
    .innerJoin(Programs, eq(SiteArchitectures.program_id, Programs.id))
    .where(and(inArray(SiteArchitectures.site_id, siteIds), eq(Programs.is_enabled, true)))
    .orderBy(asc(Programs.name));

  const programBySiteId = new Map<string, string>();
  const programs = new Map<string, { id: string; name: string }>();

  for (const row of rows) {
    programBySiteId.set(row.siteId, row.programName);
    programs.set(row.programId, {
      id: row.programId,
      name: row.programName,
    });
  }

  return {
    programBySiteId,
    filters: {
      programs: [...programs.values()].sort((left, right) => compareNames(left.name, right.name)),
    },
  };
}

export async function loadTagFilters(
  app: FastifyInstance,
): Promise<PublicSiteDirectoryMeta['filters']> {
  const rows = await app.db.read
    .select({
      id: TagDefinitions.id,
      name: TagDefinitions.name,
      machineKey: TagDefinitions.machine_key,
      tagType: TagDefinitions.tag_type,
    })
    .from(TagDefinitions)
    .where(eq(TagDefinitions.is_enabled, true))
    .orderBy(asc(TagDefinitions.tag_type), asc(TagDefinitions.name));

  const mainTags = rows
    .filter((row) => row.tagType === 'MAIN')
    .map((row) => ({ id: row.id, name: row.name }));
  const subTags = rows
    .filter((row) => row.tagType === 'SUB')
    .map((row) => ({ id: row.id, name: row.name }));
  const warningTags = rows
    .filter((row) => row.tagType === 'WARNING')
    .map((row) => ({ id: row.id, machineKey: row.machineKey ?? null, name: row.name }));

  return {
    mainTags,
    subTags,
    warningTags,
    programs: [],
  };
}

export async function loadDirectoryItems(app: FastifyInstance): Promise<PublicSiteDirectoryItem[]> {
  const siteRows = await loadPublicSiteBaseRows(app);
  return enrichPublicSites(app, siteRows);
}

export async function loadPublicSiteDirectoryMeta(
  app: FastifyInstance,
): Promise<PublicSiteDirectoryMeta> {
  const items = await loadDirectoryItems(app);
  const [filters, architecture] = await Promise.all([
    loadTagFilters(app),
    loadArchitectureFilterState(
      app,
      items.map((item) => item.id),
    ),
  ]);

  return {
    stats: {
      totalSites: items.length,
      normalSites: items.filter((item) => item.status === 'OK').length,
      abnormalSites: items.filter((item) => item.status !== 'OK').length,
      rssSites: items.filter((item) => Boolean(item.feedUrl)).length,
    },
    filters: {
      ...filters,
      programs: architecture.filters.programs,
    },
    defaults: {
      pageSize: 24,
      random: true,
      statusMode: 'normal',
    },
  };
}

export async function loadPublicSiteDirectory(
  app: FastifyInstance,
  rawQuery: PublicSiteDirectoryQuery = {},
): Promise<PublicSiteDirectoryResult> {
  const query = normalizeDirectoryQuery(rawQuery);
  const items = await loadDirectoryItems(app);
  const architecture = await loadArchitectureFilterState(
    app,
    items.map((item) => item.id),
  );
  const filtered = sortDirectoryItems(
    items.filter((item) => matchesDirectoryFilters(item, query, architecture)),
    query,
  );
  const totalItems = filtered.length;
  const totalPages = Math.max(1, Math.ceil(totalItems / query.pageSize));
  const page = Math.min(query.page, totalPages);
  const start = (page - 1) * query.pageSize;
  const pagedItems = filtered.slice(start, start + query.pageSize);

  return {
    items: pagedItems,
    pagination: {
      page,
      pageSize: query.pageSize,
      totalItems,
      totalPages,
    },
    query: {
      q: query.q,
      main: query.main,
      sub: query.sub,
      warning: query.warning,
      program: query.program,
      statusMode: query.statusMode,
      random: query.random,
      sort: query.sort,
      order: query.order,
      randomSeed: query.randomSeed,
    },
  };
}

export async function loadPublicSites(app: FastifyInstance): Promise<PublicSiteDirectoryItem[]> {
  return loadPublicSiteDirectory(app, {
    page: 1,
    pageSize: 200,
    random: false,
    sort: 'updated',
    order: 'desc',
    statusMode: 'normal',
  }).then((result) => result.items);
}

export async function resolvePublicSiteBySlug(
  app: FastifyInstance,
  slug: string,
): Promise<PublicSiteDirectoryItem | null> {
  const items = await loadDirectoryItems(app);
  return items.find((item) => matchesSiteSlug(item, slug)) ?? null;
}

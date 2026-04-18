import { FeedArticles, Programs, SiteArchitectures, Sites } from '@zhblogs/db';

import { and, desc, eq, sql } from 'drizzle-orm';
import type { FastifyInstance } from 'fastify';

import {
  type PublicSiteArticleItem,
  type PublicSiteDetail,
  type PublicSiteDetailTabPage,
} from '@/application/public/usecase/public-site.types';
import { resolvePublicSiteBySlug } from '@/application/public/usecase/public-site.usecase';
import { loadRecentPublicSiteChecks } from '@/application/public/usecase/public-site-checks.usecase';
import { listSiteWarningTags } from '@/application/sites/usecase/site-warning-tag.usecase';

export async function loadPublicSiteDetail(
  app: FastifyInstance,
  slug: string,
): Promise<PublicSiteDetail | null> {
  const site = await resolvePublicSiteBySlug(app, slug);

  if (!site) {
    return null;
  }

  const detailData = await loadPublicSiteDetailData(app, site.id);

  return {
    ...site,
    warningTags: detailData.warningTags.map((tag) => ({
      machineKey: tag.machineKey,
      name: tag.name,
      description: tag.description,
    })),
    reason: detailData.siteRow?.reason ?? null,
    feeds: mapSiteFeeds(detailData.siteRow?.feeds ?? []),
    architecture: {
      program: mapProgramDetail(detailData.architectureRow),
    },
    heartbeatChecks: detailData.heartbeatChecks,
  };
}

async function loadPublicSiteDetailData(app: FastifyInstance, siteId: string) {
  const [architectureRow, heartbeatChecks, warningTags, siteRow] = await Promise.allSettled([
    loadProgramDetail(app, siteId),
    loadRecentPublicSiteChecks(app, siteId),
    listSiteWarningTags(app, siteId),
    loadSiteDetailRow(app, siteId),
  ]);

  return {
    architectureRow: readSettledValue(architectureRow, null),
    heartbeatChecks: readSettledValue(heartbeatChecks, []),
    warningTags: readSettledValue(warningTags, []),
    siteRow: readSettledValue(siteRow, null),
  };
}

function readSettledValue<T>(result: PromiseSettledResult<T>, fallback: T): T {
  return result.status === 'fulfilled' ? result.value : fallback;
}

async function loadProgramDetail(app: FastifyInstance, siteId: string) {
  return app.db.read
    .select({
      siteId: SiteArchitectures.site_id,
      programId: Programs.id,
      programName: Programs.name,
      programIsOpenSource: Programs.is_open_source,
      websiteUrl: Programs.website_url,
      repoUrl: Programs.repo_url,
    })
    .from(SiteArchitectures)
    .innerJoin(Programs, eq(SiteArchitectures.program_id, Programs.id))
    .where(eq(SiteArchitectures.site_id, siteId))
    .limit(1)
    .then((rows) => rows[0] ?? null);
}

async function loadSiteDetailRow(app: FastifyInstance, siteId: string) {
  return app.db.read
    .select({
      reason: Sites.reason,
      feeds: Sites.feed,
    })
    .from(Sites)
    .where(eq(Sites.id, siteId))
    .limit(1)
    .then((rows) => rows[0] ?? null);
}

function mapSiteFeeds(
  feeds: Array<{
    name?: string | null;
    url?: string | null;
    type?: string | null;
    isDefault?: boolean | null;
  }>,
) {
  return feeds.flatMap((feed) =>
    feed?.url
      ? [
          {
            name: feed.name?.trim() || null,
            url: feed.url,
            type: feed.type ?? null,
            isDefault: feed.isDefault === true,
          },
        ]
      : [],
  );
}

function mapProgramDetail(
  architectureRow: {
    programId: string;
    programName: string;
    programIsOpenSource: boolean;
    websiteUrl: string | null;
    repoUrl: string | null;
  } | null,
) {
  if (!architectureRow) {
    return null;
  }

  return {
    id: architectureRow.programId,
    name: architectureRow.programName,
    isOpenSource: architectureRow.programIsOpenSource,
    websiteUrl: architectureRow.websiteUrl ?? null,
    repoUrl: architectureRow.repoUrl ?? null,
  };
}

function createPagedResult<T>(
  items: T[],
  page: number,
  pageSize: number,
  totalItems: number,
): PublicSiteDetailTabPage<T> {
  return {
    items,
    pagination: {
      page,
      pageSize,
      totalItems,
      totalPages: Math.max(1, Math.ceil(totalItems / pageSize)),
    },
  };
}

function normalizePaging(page: number, pageSize: number) {
  const normalizedPage = Math.max(1, page);
  const normalizedPageSize = Math.min(50, Math.max(10, pageSize));
  return { normalizedPage, normalizedPageSize };
}

export async function loadPublicSiteArticles(
  app: FastifyInstance,
  slug: string,
  page = 1,
  pageSize = 20,
): Promise<PublicSiteDetailTabPage<PublicSiteArticleItem> | null> {
  const site = await resolvePublicSiteBySlug(app, slug);

  if (!site) {
    return null;
  }

  const { normalizedPage, normalizedPageSize } = normalizePaging(page, pageSize);
  const totalItems = await countVisibleArticles(app, site.id);
  const totalPages = Math.max(1, Math.ceil(totalItems / normalizedPageSize));
  const currentPage = Math.min(normalizedPage, totalPages);
  const offset = (currentPage - 1) * normalizedPageSize;

  const rows = await app.db.read
    .select({
      id: FeedArticles.id,
      title: FeedArticles.title,
      articleUrl: FeedArticles.article_url,
      summary: FeedArticles.summary,
      publishedTime: FeedArticles.published_time,
      fetchedTime: FeedArticles.fetched_time,
      source: FeedArticles.source,
    })
    .from(FeedArticles)
    .where(and(eq(FeedArticles.site_id, site.id), eq(FeedArticles.visibility, 'VISIBLE')))
    .orderBy(desc(FeedArticles.published_time), desc(FeedArticles.fetched_time))
    .limit(normalizedPageSize)
    .offset(offset);

  return createPagedResult(
    rows.map(mapPublicSiteArticle),
    currentPage,
    normalizedPageSize,
    totalItems,
  );
}

async function countVisibleArticles(app: FastifyInstance, siteId: string): Promise<number> {
  return (
    (
      await app.db.read
        .select({ totalItems: sql<number>`count(*)::int` })
        .from(FeedArticles)
        .where(and(eq(FeedArticles.site_id, siteId), eq(FeedArticles.visibility, 'VISIBLE')))
    )[0]?.totalItems ?? 0
  );
}

function mapPublicSiteArticle(row: {
  id: string;
  title: string;
  articleUrl: string;
  summary: string | null;
  publishedTime: Date | null;
  fetchedTime: Date;
  source: {
    feed_name?: string | null;
    feed_url?: string | null;
    feed_type?: string | null;
  } | null;
}): PublicSiteArticleItem {
  return {
    id: row.id,
    title: row.title,
    articleUrl: row.articleUrl,
    summary: row.summary ?? null,
    publishedTime: row.publishedTime?.toISOString() ?? null,
    fetchedTime: row.fetchedTime.toISOString(),
    source: {
      feedName: row.source?.feed_name ?? null,
      feedUrl: row.source?.feed_url ?? null,
      feedType: row.source?.feed_type ?? null,
    },
  };
}

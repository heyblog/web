import { FeedArticles, Sites } from '@zhblogs/db';

import { and, desc, eq, sql } from 'drizzle-orm';
import type { FastifyInstance } from 'fastify';

import {
  createPublicCacheHash,
  loadVersionedPublicCache,
  PUBLIC_CACHE_TTL,
} from '@/application/public/usecase/public-cache.usecase';
import { createSiteSlug } from '@/application/public/usecase/public-site.directory.core';
import type {
  PublicSiteSubscriptionItem,
  PublicSiteSubscriptionResult,
  PublicSiteSubscriptionSummary,
} from '@/application/public/usecase/public-site.types';

type SubscriptionItemRow = {
  id: string;
  articleUrl: string;
  title: string;
  summary: string | null;
  publishedTime: Date | null;
  fetchedTime: Date;
  source: {
    feed_name?: string | null;
    feed_url?: string | null;
    feed_type?: string | null;
  } | null;
  siteId: string;
  siteBid: string | null;
  siteName: string;
  siteUrl: string;
};

function startOfDay(daysAgo = 0): Date {
  const current = new Date();

  current.setHours(0, 0, 0, 0);
  current.setDate(current.getDate() - daysAgo);

  return current;
}

function normalizePaging(page: number, pageSize: number) {
  const normalizedPage = Math.max(1, page);
  const normalizedPageSize = Math.min(50, Math.max(10, pageSize));

  return { normalizedPage, normalizedPageSize };
}

async function loadSubscriptionSummary(
  app: FastifyInstance,
): Promise<PublicSiteSubscriptionSummary> {
  const recentTime = sql`coalesce(${FeedArticles.published_time}, ${FeedArticles.fetched_time})`;
  const [row] = await app.db.read
    .select({
      todayArticles: sql<number>`count(*) filter (where ${recentTime} >= ${startOfDay()})::int`,
      weekArticles: sql<number>`count(*) filter (where ${recentTime} >= ${startOfDay(6)})::int`,
      totalArticles: sql<number>`count(*)::int`,
      siteCount: sql<number>`count(distinct ${FeedArticles.site_id})::int`,
    })
    .from(FeedArticles)
    .innerJoin(Sites, eq(FeedArticles.site_id, Sites.id))
    .where(and(eq(FeedArticles.visibility, 'VISIBLE'), eq(Sites.is_show, true)));

  return (
    row ?? {
      todayArticles: 0,
      weekArticles: 0,
      totalArticles: 0,
      siteCount: 0,
    }
  );
}

function mapSubscriptionItem(row: SubscriptionItemRow): PublicSiteSubscriptionItem {
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
    site: {
      id: row.siteId,
      slug: createSiteSlug({
        id: row.siteId,
        bid: row.siteBid,
        name: row.siteName,
      }),
      name: row.siteName,
      url: row.siteUrl,
    },
  };
}

export async function loadPublicSiteSubscriptions(
  app: FastifyInstance,
  page = 1,
  pageSize = 20,
): Promise<PublicSiteSubscriptionResult> {
  const { normalizedPage, normalizedPageSize } = normalizePaging(page, pageSize);
  return loadVersionedPublicCache(app, {
    namespace: 'site',
    suffix: `subscriptions:${createPublicCacheHash({
      page: normalizedPage,
      pageSize: normalizedPageSize,
    })}`,
    ttlSeconds: PUBLIC_CACHE_TTL.subscriptions,
    loader: async () => {
      const summary = await loadSubscriptionSummary(app);
      const totalPages = Math.max(1, Math.ceil(summary.totalArticles / normalizedPageSize));
      const currentPage = Math.min(normalizedPage, totalPages);
      const offset = (currentPage - 1) * normalizedPageSize;
      const recentTime = sql`coalesce(${FeedArticles.published_time}, ${FeedArticles.fetched_time})`;

      const rows = await app.db.read
        .select({
          id: FeedArticles.id,
          articleUrl: FeedArticles.article_url,
          title: FeedArticles.title,
          summary: FeedArticles.summary,
          publishedTime: FeedArticles.published_time,
          fetchedTime: FeedArticles.fetched_time,
          source: FeedArticles.source,
          siteId: Sites.id,
          siteBid: Sites.bid,
          siteName: Sites.name,
          siteUrl: Sites.url,
        })
        .from(FeedArticles)
        .innerJoin(Sites, eq(FeedArticles.site_id, Sites.id))
        .where(and(eq(FeedArticles.visibility, 'VISIBLE'), eq(Sites.is_show, true)))
        .orderBy(desc(recentTime), desc(FeedArticles.fetched_time), desc(FeedArticles.id))
        .limit(normalizedPageSize)
        .offset(offset);

      return {
        summary,
        items: rows.map(mapSubscriptionItem),
        pagination: {
          page: currentPage,
          pageSize: normalizedPageSize,
          totalItems: summary.totalArticles,
          totalPages,
        },
      };
    },
  });
}

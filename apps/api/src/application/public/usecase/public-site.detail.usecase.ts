import { FeedArticles } from '@zhblogs/db';

import { and, desc, eq, sql } from 'drizzle-orm';
import type { FastifyInstance } from 'fastify';

import {
  loadVersionedPublicCache,
  PUBLIC_CACHE_TTL,
} from '@/application/public/usecase/public-cache.usecase';
import {
  type PublicSiteArticleItem,
  type PublicSiteDetail,
  type PublicSiteDetailTabPage,
} from '@/application/public/usecase/public-site.types';
import {
  resolvePublicSiteBySlug,
  resolvePublicSiteDetailBySlug,
} from '@/application/public/usecase/public-site.usecase';
import { loadRecentPublicSiteChecks } from '@/application/public/usecase/public-site-checks.usecase';

export async function loadPublicSiteDetail(
  app: FastifyInstance,
  slug: string,
): Promise<PublicSiteDetail | null> {
  return loadVersionedPublicCache(app, {
    namespace: 'site',
    suffix: `detail:${slug}`,
    ttlSeconds: PUBLIC_CACHE_TTL.detail,
    loader: async () => {
      const [detail, heartbeatChecks] = await Promise.all([
        resolvePublicSiteDetailBySlug(app, slug, []),
        loadRecentPublicSiteChecks(app, slug),
      ]);

      if (!detail) {
        return null;
      }

      return {
        ...detail,
        heartbeatChecks,
      };
    },
  });
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
  const { normalizedPage, normalizedPageSize } = normalizePaging(page, pageSize);
  return loadVersionedPublicCache(app, {
    namespace: 'site',
    suffix: `articles:${slug}:${normalizedPage}:${normalizedPageSize}`,
    ttlSeconds: PUBLIC_CACHE_TTL.articles,
    loader: async () => {
      const totalItems = await countVisibleArticles(app, slug);

      if (totalItems === 0) {
        const site = await resolvePublicSiteBySlug(app, slug);

        if (!site) {
          return null;
        }
      }

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
        .where(and(eq(FeedArticles.site_id, slug), eq(FeedArticles.visibility, 'VISIBLE')))
        .orderBy(desc(FeedArticles.published_time), desc(FeedArticles.fetched_time))
        .limit(normalizedPageSize)
        .offset(offset);

      return createPagedResult(
        rows.map(mapPublicSiteArticle),
        currentPage,
        normalizedPageSize,
        totalItems,
      );
    },
  });
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

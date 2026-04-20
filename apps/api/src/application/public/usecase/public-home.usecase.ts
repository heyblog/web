import { Announcements, SiteFeedArticleStats, Sites } from '@zhblogs/db';

import { and, desc, eq, gt, isNull, lte, or, sql } from 'drizzle-orm';
import type { FastifyInstance } from 'fastify';

import {
  loadVersionedPublicCache,
  PUBLIC_CACHE_TTL,
} from '@/application/public/usecase/public-cache.usecase';

function startOfToday(): Date {
  const current = new Date();

  current.setHours(0, 0, 0, 0);

  return current;
}

function readQueryRows<T extends object>(value: unknown): T[] {
  if (Array.isArray(value)) {
    return value as T[];
  }

  if (value && typeof value === 'object' && 'rows' in value && Array.isArray(value.rows)) {
    return value.rows as T[];
  }

  return [];
}

export async function loadPublicHomeSummary(app: FastifyInstance) {
  return loadVersionedPublicCache(app, {
    namespace: 'site',
    suffix: 'home:summary',
    ttlSeconds: PUBLIC_CACHE_TTL.homeSummary,
    loader: async () => {
      const query = await app.db.read.execute(sql<{
        totalSites: number;
        featuredSites: number;
        todayUpdates: number;
      }>`
        select
          count(*) filter (where ${Sites.is_show} = true)::int as "totalSites",
          count(*) filter (where ${Sites.is_show} = true and ${Sites.recommend} = true)::int as "featuredSites",
          count(*) filter (
            where ${Sites.is_show} = true
              and ${SiteFeedArticleStats.latest_published_time} >= ${startOfToday()}
          )::int as "todayUpdates"
        from ${Sites}
        left join ${SiteFeedArticleStats} on ${SiteFeedArticleStats.site_id} = ${Sites.id}
      `);
      const row = readQueryRows<{
        totalSites: number;
        featuredSites: number;
        todayUpdates: number;
      }>(query)[0];

      return {
        totalSites: row?.totalSites ?? 0,
        featuredSites: row?.featuredSites ?? 0,
        todayUpdates: row?.todayUpdates ?? 0,
      };
    },
  });
}

export async function loadCurrentAnnouncement(app: FastifyInstance) {
  return loadVersionedPublicCache(app, {
    namespace: 'announcement',
    suffix: 'current',
    ttlSeconds: PUBLIC_CACHE_TTL.currentAnnouncement,
    loader: async () => {
      const now = new Date();
      const [row] = await app.db.read
        .select({
          id: Announcements.id,
          title: Announcements.title,
          content: Announcements.content,
          publishTime: Announcements.publish_time,
        })
        .from(Announcements)
        .where(
          and(
            eq(Announcements.status, 'PUBLISHED'),
            lte(Announcements.publish_time, now),
            or(isNull(Announcements.expire_time), gt(Announcements.expire_time, now)),
          ),
        )
        .orderBy(desc(Announcements.publish_time), desc(Announcements.created_time))
        .limit(1);

      if (!row) {
        return null;
      }

      return {
        ...row,
        publishTime: row.publishTime ? row.publishTime.toISOString() : null,
      };
    },
  });
}

function normalizeAnnouncementPagination(page: number, pageSize: number, totalItems: number) {
  const normalizedPageSize = Math.min(50, Math.max(1, pageSize));
  const totalPages = Math.max(1, Math.ceil(totalItems / normalizedPageSize));
  const normalizedPage = Math.min(totalPages, Math.max(1, page));

  return {
    page: normalizedPage,
    pageSize: normalizedPageSize,
    totalItems,
    totalPages,
  };
}

export async function loadPublicAnnouncements(app: FastifyInstance, page = 1, pageSize = 20) {
  const normalizedPageSize = Math.min(50, Math.max(1, pageSize));
  const normalizedPage = Math.max(1, page);

  return loadVersionedPublicCache(app, {
    namespace: 'announcement',
    suffix: `archive:${normalizedPage}:${normalizedPageSize}`,
    ttlSeconds: PUBLIC_CACHE_TTL.announcementArchive,
    loader: async () => {
      const now = new Date();
      const filters = and(
        or(eq(Announcements.status, 'PUBLISHED'), eq(Announcements.status, 'EXPIRED')),
        lte(Announcements.publish_time, now),
      );

      const [countRow] = await app.db.read
        .select({
          total: sql<number>`count(*)::int`,
        })
        .from(Announcements)
        .where(filters);

      const pagination = normalizeAnnouncementPagination(
        normalizedPage,
        normalizedPageSize,
        countRow?.total ?? 0,
      );
      const rows = await app.db.read
        .select({
          id: Announcements.id,
          title: Announcements.title,
          content: Announcements.content,
          status: Announcements.status,
          publishTime: Announcements.publish_time,
          expireTime: Announcements.expire_time,
        })
        .from(Announcements)
        .where(filters)
        .orderBy(desc(Announcements.publish_time), desc(Announcements.created_time))
        .limit(pagination.pageSize)
        .offset((pagination.page - 1) * pagination.pageSize);

      return {
        items: rows.map((row) => ({
          ...row,
          publishTime: row.publishTime ? row.publishTime.toISOString() : null,
          expireTime: row.expireTime ? row.expireTime.toISOString() : null,
        })),
        pagination,
      };
    },
  });
}

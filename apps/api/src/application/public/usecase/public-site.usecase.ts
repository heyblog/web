import type { FastifyInstance } from 'fastify';

import {
  createPublicCacheHash,
  loadVersionedPublicCache,
  PUBLIC_CACHE_TTL,
} from '@/application/public/usecase/public-cache.usecase';
import { normalizeDirectoryQuery } from '@/application/public/usecase/public-site.directory-query';
import {
  countFilteredDirectoryItems,
  loadDirectoryDetailBySlug,
  loadDirectoryItemBySlug,
  loadDirectoryStats,
  loadFilteredDirectoryItems,
  loadProgramFilters,
  loadTagFilters,
} from '@/application/public/usecase/public-site.query';
import type {
  DirectoryState,
  PublicSiteDetail,
  PublicSiteDirectoryItem,
  PublicSiteDirectoryMeta,
  PublicSiteDirectoryQuery,
  PublicSiteDirectoryResult,
} from '@/application/public/usecase/public-site.types';

export type {
  PublicSiteDirectoryItem,
  PublicSiteDirectoryMeta,
  PublicSiteDirectoryQuery,
  PublicSiteDirectoryResult,
  PublicSiteWarningTag,
} from '@/application/public/usecase/public-site.types';

const PUBLIC_DIRECTORY_DEFAULTS = {
  pageSize: 24,
  random: true,
  statusMode: 'normal' as const,
};

async function loadPublicSiteDirectoryResult(
  app: FastifyInstance,
  query: DirectoryState,
): Promise<PublicSiteDirectoryResult> {
  const totalItems = await countFilteredDirectoryItems(app, query);
  const totalPages = Math.max(1, Math.ceil(totalItems / query.pageSize));
  const page = Math.min(query.page, totalPages);
  const items = await loadFilteredDirectoryItems(app, query, page);

  return {
    items,
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

export async function loadPublicSiteDirectoryMeta(
  app: FastifyInstance,
): Promise<PublicSiteDirectoryMeta> {
  return loadVersionedPublicCache(app, {
    namespace: 'site',
    suffix: 'meta',
    ttlSeconds: PUBLIC_CACHE_TTL.meta,
    loader: async () => {
      const [stats, filters, programs] = await Promise.all([
        loadDirectoryStats(app),
        loadTagFilters(app),
        loadProgramFilters(app),
      ]);

      return {
        stats,
        filters: {
          ...filters,
          programs,
        },
        defaults: PUBLIC_DIRECTORY_DEFAULTS,
      };
    },
  });
}

export async function loadPublicSiteDirectory(
  app: FastifyInstance,
  rawQuery: PublicSiteDirectoryQuery = {},
): Promise<PublicSiteDirectoryResult> {
  const query = normalizeDirectoryQuery(rawQuery);

  return loadVersionedPublicCache(app, {
    namespace: 'site',
    suffix: `list:${createPublicCacheHash(query)}`,
    ttlSeconds: PUBLIC_CACHE_TTL.list,
    loader: () => loadPublicSiteDirectoryResult(app, query),
  });
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
  return loadDirectoryItemBySlug(app, slug);
}

export async function resolvePublicSiteDetailBySlug(
  app: FastifyInstance,
  slug: string,
  heartbeatChecks: PublicSiteDetail['heartbeatChecks'],
): Promise<PublicSiteDetail | null> {
  return loadDirectoryDetailBySlug(app, slug, heartbeatChecks);
}

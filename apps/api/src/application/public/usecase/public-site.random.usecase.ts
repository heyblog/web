import type { FastifyInstance } from 'fastify';

import {
  createPublicCacheHash,
  loadVersionedPublicCache,
  PUBLIC_CACHE_TTL,
} from '@/application/public/usecase/public-cache.usecase';
import { compareNames } from '@/application/public/usecase/public-site.directory.core';
import {
  loadRandomDirectoryItem,
  loadTagFilters,
} from '@/application/public/usecase/public-site.query';
import type {
  PublicSiteRandomFailureReason,
  PublicSiteRandomFilters,
  PublicSiteRandomResult,
} from '@/application/public/usecase/public-site.types';

const RANDOM_ALLOWED_PARAMS = new Set(['recommend', 'type']);

function createSiteRandomSeed(date = new Date()): string {
  return `site-go:${date.toISOString()}`;
}

function parseRandomFilters(url: URL): PublicSiteRandomFilters {
  return {
    recommend: url.searchParams.get('recommend') === 'true',
    type: url.searchParams.get('type')?.trim() ?? '',
  };
}

function resolveRandomFailureReason(
  url: URL,
  availableTypes: string[],
): PublicSiteRandomFailureReason | null {
  const { searchParams } = url;

  for (const key of RANDOM_ALLOWED_PARAMS) {
    if (searchParams.getAll(key).length > 1) {
      return 'DUPLICATE_PARAM';
    }
  }

  for (const [key] of searchParams.entries()) {
    if (!RANDOM_ALLOWED_PARAMS.has(key)) {
      return 'UNKNOWN_PARAM';
    }
  }

  if (searchParams.has('recommend') && searchParams.get('recommend') !== 'true') {
    const type = searchParams.get('type')?.trim() ?? '';
    const invalidType = searchParams.has('type') && (!type || !availableTypes.includes(type));

    return invalidType ? 'INVALID_PARAMS' : 'INVALID_RECOMMEND';
  }

  if (searchParams.has('type')) {
    const type = searchParams.get('type')?.trim() ?? '';

    if (!type || !availableTypes.includes(type)) {
      return 'INVALID_TYPE';
    }
  }

  return null;
}

export async function loadPublicSiteRandom(
  app: FastifyInstance,
  rawUrl: string,
): Promise<PublicSiteRandomResult> {
  const url = new URL(rawUrl, 'https://www.zhblogs.net');
  const filters = parseRandomFilters(url);
  const availableTypes = (await loadTagFilters(app)).mainTags
    .map((item) => item.name)
    .sort(compareNames);
  const failureReason = resolveRandomFailureReason(url, availableTypes);

  if (failureReason) {
    return {
      site: null,
      availableTypes,
      filters,
      failureReason,
    };
  }

  const seed = createSiteRandomSeed();

  return loadVersionedPublicCache(app, {
    namespace: 'site',
    suffix: `random:${createPublicCacheHash({ filters, seed })}`,
    ttlSeconds: PUBLIC_CACHE_TTL.random,
    loader: async () => {
      const site = await loadRandomDirectoryItem(app, {
        seed,
        recommend: filters.recommend,
        type: filters.type,
      });

      return {
        site,
        availableTypes,
        filters,
        failureReason: site ? null : 'NO_MATCH',
      };
    },
  });
}

import { createHash } from 'node:crypto';

import type { FastifyInstance } from 'fastify';

import {
  incrementCacheKey,
  readCacheJsonWithHit,
  readCacheString,
  writeCacheJson,
} from '@/infrastructure/app/cache/app-cache.store';

const PUBLIC_SITE_VERSION_KEY = 'public:site:version';
const PUBLIC_ANNOUNCEMENT_VERSION_KEY = 'public:announcement:version';

export const PUBLIC_CACHE_TTL = {
  list: 300,
  meta: 300,
  detail: 300,
  random: 300,
  articles: 120,
  checks: 120,
  subscriptions: 120,
  homeSummary: 300,
  currentAnnouncement: 60,
  announcementArchive: 120,
} as const;

type PublicCacheNamespace = 'site' | 'announcement';

const readNamespaceVersion = async (
  app: FastifyInstance,
  namespace: PublicCacheNamespace,
): Promise<string> => {
  const key = namespace === 'site' ? PUBLIC_SITE_VERSION_KEY : PUBLIC_ANNOUNCEMENT_VERSION_KEY;
  return (await readCacheString(app.db.cache, key)) ?? '1';
};

const buildNamespaceKey = (
  namespace: PublicCacheNamespace,
  version: string,
  suffix: string,
): string => `public:${namespace}:v${version}:${suffix}`;

export const createPublicCacheHash = (value: unknown): string =>
  createHash('sha256').update(JSON.stringify(value)).digest('hex').slice(0, 16);

export async function readVersionedPublicCache<T>(
  app: FastifyInstance,
  namespace: PublicCacheNamespace,
  suffix: string,
): Promise<{ hit: boolean; value: T | null; key: string }> {
  const version = await readNamespaceVersion(app, namespace);
  const key = buildNamespaceKey(namespace, version, suffix);
  const cached = await readCacheJsonWithHit<T>(app.db.cache, key);

  return {
    hit: cached.hit,
    value: cached.hit ? cached.value : null,
    key,
  };
}

export async function loadVersionedPublicCache<T>(
  app: FastifyInstance,
  options: {
    namespace: PublicCacheNamespace;
    suffix: string;
    ttlSeconds: number;
    loader: () => Promise<T>;
  },
): Promise<T> {
  const cached = await readVersionedPublicCache<T>(app, options.namespace, options.suffix);

  if (cached.hit) {
    return cached.value as T;
  }

  const value = await options.loader();
  await writeCacheJson(app.db.cache, cached.key, value, options.ttlSeconds);
  return value;
}

export async function invalidatePublicSiteCache(app: FastifyInstance): Promise<void> {
  await incrementCacheKey(app.db.cache, PUBLIC_SITE_VERSION_KEY);
}

export async function invalidateAnnouncementCache(app: FastifyInstance): Promise<void> {
  await incrementCacheKey(app.db.cache, PUBLIC_ANNOUNCEMENT_VERSION_KEY);
}

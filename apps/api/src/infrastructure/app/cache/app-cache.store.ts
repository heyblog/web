import type { CacheClient } from '@/shared/runtime/types/app-dependencies.types';

type CacheJsonReadResult<T> =
  | {
      hit: false;
      value: null;
    }
  | {
      hit: true;
      value: T;
    };

const getWithCommandFallback = async (cache: CacheClient, key: string): Promise<unknown> => {
  if (cache.get) {
    return cache.get(key);
  }

  if (cache.customCommand) {
    return cache.customCommand(['GET', key]);
  }

  return null;
};

const setWithCommandFallback = async (
  cache: CacheClient,
  key: string,
  value: string,
  ttlSeconds?: number,
): Promise<void> => {
  if (cache.customCommand) {
    const args = ['SET', key, value];

    if (ttlSeconds && ttlSeconds > 0) {
      args.push('EX', String(ttlSeconds));
    }

    await cache.customCommand(args);
    return;
  }

  if (cache.set) {
    await cache.set(key, value);
  }
};

const deleteWithCommandFallback = async (
  cache: CacheClient,
  keys: string[] | string,
): Promise<void> => {
  const targets = Array.isArray(keys) ? keys : [keys];

  if (targets.length === 0) {
    return;
  }

  if (cache.delete) {
    await cache.delete(targets);
    return;
  }

  if (cache.customCommand) {
    await cache.customCommand(['DEL', ...targets]);
  }
};

export const readCacheString = async (
  cache: CacheClient | undefined,
  key: string,
): Promise<string | null> => {
  if (!cache) {
    return null;
  }

  const result = await getWithCommandFallback(cache, key);
  return typeof result === 'string' ? result : null;
};

export const readCacheJsonWithHit = async <T>(
  cache: CacheClient | undefined,
  key: string,
): Promise<CacheJsonReadResult<T>> => {
  const result = await readCacheString(cache, key);

  if (result === null) {
    return {
      hit: false,
      value: null,
    };
  }

  return {
    hit: true,
    value: JSON.parse(result) as T,
  };
};

export const readCacheJson = async <T>(
  cache: CacheClient | undefined,
  key: string,
): Promise<T | null> => {
  const result = await readCacheJsonWithHit<T>(cache, key);
  return result.hit ? result.value : null;
};

export const writeCacheString = async (
  cache: CacheClient | undefined,
  key: string,
  value: string,
  ttlSeconds?: number,
): Promise<void> => {
  if (!cache) {
    return;
  }

  await setWithCommandFallback(cache, key, value, ttlSeconds);
};

export const writeCacheJson = async (
  cache: CacheClient | undefined,
  key: string,
  value: unknown,
  ttlSeconds?: number,
): Promise<void> => {
  await writeCacheString(cache, key, JSON.stringify(value), ttlSeconds);
};

export const removeCacheKeys = async (
  cache: CacheClient | undefined,
  keys: string[] | string,
): Promise<void> => {
  if (!cache) {
    return;
  }

  await deleteWithCommandFallback(cache, keys);
};

export const incrementCacheKey = async (
  cache: CacheClient | undefined,
  key: string,
): Promise<number | null> => {
  if (!cache?.customCommand) {
    return null;
  }

  const result = await cache.customCommand(['INCR', key]);
  return typeof result === 'number' ? result : Number(result);
};

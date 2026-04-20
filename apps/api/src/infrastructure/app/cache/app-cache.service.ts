import fp from 'fastify-plugin';

import type { CacheClient } from '@/shared/runtime/types/app-dependencies.types';

const parseValkeyUrl = (url: string): { host: string; port: number; useTLS: boolean } => {
  const parsed = new URL(url);

  return {
    host: parsed.hostname,
    port: parsed.port ? Number(parsed.port) : 6379,
    useTLS: parsed.protocol === 'rediss:',
  };
};

export const cachePlugin = fp(
  async (app) => {
    if (app.bootstrapOptions.disableExternalServices) {
      const stringStore = new Map<string, string>();
      const expiryStore = new Map<string, number>();
      const sortedSetStore = new Map<string, Map<string, number>>();

      const cleanupExpiredKey = (key: string) => {
        const expiresAt = expiryStore.get(key);

        if (typeof expiresAt === 'number' && expiresAt <= Date.now()) {
          stringStore.delete(key);
          sortedSetStore.delete(key);
          expiryStore.delete(key);
        }
      };

      const setStringValue = (key: string, value: string, ttlSeconds?: number) => {
        stringStore.set(key, value);

        if (ttlSeconds && ttlSeconds > 0) {
          expiryStore.set(key, Date.now() + ttlSeconds * 1_000);
          return;
        }

        expiryStore.delete(key);
      };

      const deleteKey = (key: string) => {
        cleanupExpiredKey(key);

        let deleted = 0;

        if (stringStore.delete(key)) {
          deleted += 1;
        }

        if (sortedSetStore.delete(key)) {
          deleted += 1;
        }

        expiryStore.delete(key);
        return deleted;
      };

      const getSortedSet = (key: string) => {
        let members = sortedSetStore.get(key);

        if (!members) {
          members = new Map<string, number>();
          sortedSetStore.set(key, members);
        }

        return members;
      };

      const parseSortedSetBound = (value: string, fallback: number) => {
        if (value === '-inf') {
          return Number.NEGATIVE_INFINITY;
        }

        if (value === '+inf') {
          return Number.POSITIVE_INFINITY;
        }

        const parsed = Number(value);

        return Number.isFinite(parsed) ? parsed : fallback;
      };

      const mockClient = {
        ping: async () => 'PONG',
        get: async (key: string) => {
          cleanupExpiredKey(key);
          return stringStore.get(key) ?? null;
        },
        set: async (key: string, value: string) => {
          setStringValue(key, value);
          return 'OK';
        },
        delete: async (keys: string[] | string) => {
          const targets = Array.isArray(keys) ? keys : [keys];
          let deleted = 0;

          for (const key of targets) {
            deleted += deleteKey(key);
          }

          return deleted;
        },
        customCommand: async (args: string[]) => {
          const [command, ...rest] = args;

          switch (command?.toUpperCase()) {
            case 'GET':
              cleanupExpiredKey(rest[0] ?? '');
              return stringStore.get(rest[0] ?? '') ?? null;
            case 'SET': {
              const ttlIndex = rest.findIndex((value) => value.toUpperCase() === 'EX');
              const ttlSeconds =
                ttlIndex >= 0 && rest[ttlIndex + 1] ? Number(rest[ttlIndex + 1]) : undefined;

              setStringValue(rest[0] ?? '', rest[1] ?? '', ttlSeconds);
              return 'OK';
            }
            case 'DEL': {
              let deleted = 0;

              for (const key of rest) {
                deleted += deleteKey(key);
              }

              return deleted;
            }
            case 'INCR': {
              const key = rest[0] ?? '';

              cleanupExpiredKey(key);

              const nextValue = Number(stringStore.get(key) ?? '0') + 1;

              setStringValue(key, String(nextValue));
              return nextValue;
            }
            case 'ZADD': {
              const [key, scoreValue, member] = rest;

              if (!key || !scoreValue || !member) {
                throw new Error('invalid ZADD args for mock client');
              }

              const score = Number(scoreValue);
              const members = getSortedSet(key);
              const isNewMember = !members.has(member);

              members.set(member, score);

              return isNewMember ? 1 : 0;
            }
            case 'ZCOUNT': {
              const [key, minValue, maxValue] = rest;

              if (!key || !minValue || !maxValue) {
                throw new Error('invalid ZCOUNT args for mock client');
              }

              const members = getSortedSet(key);
              const min = parseSortedSetBound(minValue, Number.NEGATIVE_INFINITY);
              const max = parseSortedSetBound(maxValue, Number.POSITIVE_INFINITY);
              let count = 0;

              for (const score of members.values()) {
                if (score >= min && score <= max) {
                  count += 1;
                }
              }

              return count;
            }
            case 'ZREMRANGEBYSCORE': {
              const [key, minValue, maxValue] = rest;

              if (!key || !minValue || !maxValue) {
                throw new Error('invalid ZREMRANGEBYSCORE args for mock client');
              }

              const members = getSortedSet(key);
              const min = parseSortedSetBound(minValue, Number.NEGATIVE_INFINITY);
              const max = parseSortedSetBound(maxValue, Number.POSITIVE_INFINITY);
              let deleted = 0;

              for (const [member, score] of members.entries()) {
                if (score >= min && score <= max) {
                  members.delete(member);
                  deleted += 1;
                }
              }

              return deleted;
            }
            default:
              throw new Error(`unsupported cache command in mock client: ${command}`);
          }
        },
        close: () => undefined,
      } as unknown as CacheClient;

      app.db.cache = mockClient;
      return;
    }

    const { host, port, useTLS } = parseValkeyUrl(app.config.VALKEY_URL);
    const { GlideClient } = await import('@valkey/valkey-glide');
    const client = (await GlideClient.createClient({
      addresses: [{ host, port }],
      clientName: `zhblogs-api-${app.config.NODE_ENV}`,
      requestTimeout: 1_000,
      useTLS,
    })) as unknown as CacheClient;

    app.db.cache = client;
    app.log.info({ host, port, useTLS }, 'cache client connected');

    app.addHook('onClose', async () => {
      client.close();
    });
  },
  { name: 'cache', dependencies: ['config', 'drizzle'] },
);

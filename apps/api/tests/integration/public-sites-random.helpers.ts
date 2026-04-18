import {
  SiteAccessCounters,
  SiteFeedArticleStats,
  Sites,
  SiteTags,
  SiteWarningTags,
  TagDefinitions,
} from '@zhblogs/db';

import { expect, vi } from 'vitest';

import { type createTestApp } from '@tests/create-test-app';

export function mockRandomQueries(
  app: ReturnType<typeof createTestApp>,
  options: {
    sites: unknown[];
    stats?: unknown[];
    access?: unknown[];
    tags?: unknown[];
    warnings?: unknown[];
    mainTags?: unknown[];
  },
) {
  app.db.read.select = vi.fn(() => ({
    from(table: unknown) {
      if (table === TagDefinitions) {
        return {
          where: vi.fn(() => ({
            orderBy: vi.fn(async () => options.mainTags ?? []),
          })),
        };
      }

      if (table === Sites) {
        return {
          where: vi.fn(() => ({
            orderBy: vi.fn(async () => options.sites),
          })),
        };
      }

      if (table === SiteFeedArticleStats) {
        return {
          where: vi.fn(async () => options.stats ?? []),
        };
      }

      if (table === SiteAccessCounters) {
        return {
          where: vi.fn(async () => options.access ?? []),
        };
      }

      if (table === SiteTags) {
        return {
          innerJoin: vi.fn((joinedTable: unknown) => {
            expect(joinedTable).toBe(TagDefinitions);

            return {
              where: vi.fn(async () => options.tags ?? []),
            };
          }),
        };
      }

      if (table === SiteWarningTags) {
        return {
          innerJoin: vi.fn((joinedTable: unknown) => {
            expect(joinedTable).toBe(TagDefinitions);

            return {
              where: vi.fn(() => ({
                orderBy: vi.fn(async () => options.warnings ?? []),
              })),
            };
          }),
        };
      }

      throw new Error('unexpected read query');
    },
  })) as unknown as typeof app.db.read.select;
}

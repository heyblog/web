import { TagDefinitions } from '@zhblogs/db';

import { expect, vi } from 'vitest';

import type { createTestApp } from '@tests/create-test-app';

export function mockRandomQueries(
  app: ReturnType<typeof createTestApp>,
  options: {
    randomSite?: unknown | null;
    mainTags?: Array<{
      id: string;
      name: string;
      machineKey: string | null;
      tagType: 'MAIN' | 'SUB' | 'WARNING';
    }>;
  },
) {
  app.db.read.select = vi.fn(() => ({
    from(table: unknown) {
      expect(table).toBe(TagDefinitions);

      return {
        where: vi.fn(() => ({
          orderBy: vi.fn(async () => options.mainTags ?? []),
        })),
      };
    },
  })) as unknown as typeof app.db.read.select;

  app.db.read.execute = vi
    .fn()
    .mockResolvedValueOnce(
      options.randomSite ? [options.randomSite] : [],
    ) as unknown as typeof app.db.read.execute;
}

import { beforeEach, describe, expect, it, vi } from 'vitest';

import { resolvePublicSiteBySlug } from '@/application/public/usecase/public-site.usecase';
import {
  loadPublicSiteChecks,
  loadRecentPublicSiteChecks,
} from '@/application/public/usecase/public-site-checks.usecase';

vi.mock('@/application/public/usecase/public-site.usecase', () => ({
  resolvePublicSiteBySlug: vi.fn(),
}));

function createAppMock() {
  return {
    db: {
      read: {
        execute: vi.fn(),
      },
    },
    log: {
      warn: vi.fn(),
    },
  };
}

describe('public site checks usecase', () => {
  beforeEach(() => {
    vi.mocked(resolvePublicSiteBySlug).mockReset();
  });

  it('falls back to summary rows when expanded check rows fail', async () => {
    const app = createAppMock();
    vi.mocked(resolvePublicSiteBySlug).mockResolvedValue({ id: 'site-1' } as never);
    app.db.read.execute
      .mockResolvedValueOnce([{ totalItems: 2 }])
      .mockRejectedValueOnce(new Error('invalid probe summary'))
      .mockResolvedValueOnce([
        {
          id: 'run-1:SUMMARY',
          region: 'CN',
          result: 'SUCCESS',
          statusCode: null,
          responseTimeMs: 180,
          durationMs: 220,
          message: null,
          finalUrl: 'https://alpha.example',
          contentVerified: true,
          checkTime: new Date('2026-04-16T03:00:00.000Z'),
        },
      ]);

    const result = await loadPublicSiteChecks(app as never, 'site-1', 1, 20);

    expect(result).toEqual({
      items: [
        {
          id: 'run-1:SUMMARY',
          region: 'CN',
          result: 'SUCCESS',
          statusCode: null,
          responseTimeMs: 180,
          durationMs: 220,
          message: null,
          finalUrl: 'https://alpha.example',
          contentVerified: true,
          checkTime: '2026-04-16T03:00:00.000Z',
        },
      ],
      pagination: {
        page: 1,
        pageSize: 20,
        totalItems: 2,
        totalPages: 1,
      },
    });
    expect(app.log.warn).toHaveBeenCalledTimes(1);
  });

  it('falls back to summary rows for heartbeat checks when expanded rows are unavailable', async () => {
    const app = createAppMock();
    app.db.read.execute
      .mockResolvedValueOnce([{ totalItems: 1 }])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([
        {
          id: 'run-2:SUMMARY',
          region: 'GLOBAL',
          result: 'BLOCKED',
          statusCode: null,
          responseTimeMs: null,
          durationMs: 350,
          message: 'blocked',
          finalUrl: null,
          contentVerified: false,
          checkTime: new Date('2026-04-16T04:00:00.000Z'),
        },
      ]);

    const result = await loadRecentPublicSiteChecks(app as never, 'site-1', 10);

    expect(result).toEqual([
      {
        id: 'run-2:SUMMARY',
        region: 'GLOBAL',
        result: 'BLOCKED',
        statusCode: null,
        responseTimeMs: null,
        durationMs: 350,
        message: 'blocked',
        finalUrl: null,
        contentVerified: false,
        checkTime: '2026-04-16T04:00:00.000Z',
      },
    ]);
  });
});

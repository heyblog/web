import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  mapDirectoryItemToSiteCardEntry,
  resolveUpdatedLabel,
  resolveUpdatedTone,
} from '@/application/site/site-card.shared';

describe('site card updated metadata', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-04-16T00:00:00.000Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('maps updated time to expected color bands', () => {
    expect(resolveUpdatedTone('2026-04-15T00:00:00.000Z')).toBe('emerald');
    expect(resolveUpdatedTone('2026-04-01T00:00:00.000Z')).toBe('amber');
    expect(resolveUpdatedTone('2026-02-15T00:00:00.000Z')).toBe('blue');
    expect(resolveUpdatedTone('2025-10-01T00:00:00.000Z')).toBe('stone');
    expect(resolveUpdatedTone(null)).toBeNull();
  });

  it('returns stable updated labels for empty and recent values', () => {
    expect(resolveUpdatedLabel(null)).toBeNull();
    expect(resolveUpdatedLabel('2026-04-16T00:00:00.000Z')).toBe('今天更新');
    expect(resolveUpdatedLabel('2026-04-13T00:00:00.000Z')).toBe('3 天前更新');
    expect(resolveUpdatedLabel('2024-04-16T00:00:00.000Z')).toBe('2 年前更新');
  });

  it('hides updated metadata when rss is missing', () => {
    const entry = mapDirectoryItemToSiteCardEntry({
      id: 'site-1',
      bid: null,
      slug: 'site-1',
      name: 'Alpha',
      url: 'https://alpha.example',
      sign: 'alpha',
      feedUrl: null,
      sitemap: null,
      linkPage: null,
      featured: false,
      status: 'OK',
      accessScope: 'ALL',
      joinTime: '2026-04-01T00:00:00.000Z',
      updateTime: '2026-04-10T00:00:00.000Z',
      latestPublishedTime: '2026-04-15T00:00:00.000Z',
      articleCount: 12,
      visitCount: 18,
      primaryTag: '技术',
      subTags: [],
      warningTags: [],
    });

    expect(entry.updatedLabel).toBeNull();
    expect(entry.updatedTone).toBeNull();
  });
});

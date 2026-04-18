import { describe, expect, it } from 'vitest';

import { sortDirectoryItems } from '@/application/public/usecase/public-site.directory-filter';
import { normalizeDirectoryQuery } from '@/application/public/usecase/public-site.directory-query';
import type { PublicSiteDirectoryItem } from '@/application/public/usecase/public-site.types';

const createItem = (overrides: Partial<PublicSiteDirectoryItem>): PublicSiteDirectoryItem => ({
  id: 'site-1',
  bid: null,
  slug: 'site-1',
  name: 'Alpha',
  url: 'https://alpha.example',
  sign: '',
  feedUrl: null,
  sitemap: null,
  linkPage: null,
  featured: false,
  status: 'OK',
  accessScope: 'ALL',
  joinTime: '2026-03-01T00:00:00.000Z',
  updateTime: '2026-04-10T00:00:00.000Z',
  latestPublishedTime: '2026-04-10T00:00:00.000Z',
  articleCount: 0,
  visitCount: 0,
  primaryTag: '技术',
  subTags: [],
  warningTags: [],
  ...overrides,
});

const items = [
  createItem({
    id: 'site-1',
    slug: 'site-1',
    name: 'Alpha',
    updateTime: '2026-04-15T00:00:00.000Z',
    latestPublishedTime: '2026-04-01T00:00:00.000Z',
  }),
  createItem({
    id: 'site-2',
    slug: 'site-2',
    name: 'Beta',
    updateTime: '2026-04-02T00:00:00.000Z',
    latestPublishedTime: '2026-04-14T00:00:00.000Z',
  }),
];

describe('public site directory sorting', () => {
  it('disables random mode when sort is provided', () => {
    const query = normalizeDirectoryQuery({
      random: true,
      sort: 'updated',
      order: 'desc',
    });

    expect(query.sort).toBe('updated');
    expect(query.random).toBe(false);
  });

  it('sorts updated results by latest published time first', () => {
    const query = normalizeDirectoryQuery({
      random: false,
      sort: 'updated',
      order: 'desc',
    });

    const sorted = sortDirectoryItems(items, query);
    expect(sorted.map((item) => item.id)).toEqual(['site-2', 'site-1']);
  });

  it('keeps source order when random and sort are both disabled', () => {
    const query = normalizeDirectoryQuery({
      random: false,
    });

    const sorted = sortDirectoryItems(items, query);
    expect(sorted.map((item) => item.id)).toEqual(['site-1', 'site-2']);
  });
});

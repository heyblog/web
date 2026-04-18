import { describe, expect, it } from 'vitest';

import { matchesSiteSlug } from '@/application/public/usecase/public-site.directory.core';

describe('public site directory core', () => {
  it('matches both public slug and fallback id', () => {
    expect(matchesSiteSlug({ id: 'site-1', slug: 'alpha-blog' }, 'alpha-blog')).toBe(true);
    expect(matchesSiteSlug({ id: 'site-1', slug: 'alpha-blog' }, 'site-1')).toBe(true);
    expect(matchesSiteSlug({ id: 'site-1', slug: 'alpha-blog' }, 'beta-blog')).toBe(false);
  });
});

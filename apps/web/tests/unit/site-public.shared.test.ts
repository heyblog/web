import { describe, expect, it } from 'vitest';

import { matchesSiteSlug } from '@/application/site/site-public.server';

describe('site public shared helpers', () => {
  it('matches both public slug and fallback id', () => {
    expect(matchesSiteSlug({ id: 'site-1', slug: 'alpha-blog' }, 'alpha-blog')).toBe(true);
    expect(matchesSiteSlug({ id: 'site-1', slug: 'alpha-blog' }, 'site-1')).toBe(true);
    expect(matchesSiteSlug({ id: 'site-1', slug: 'alpha-blog' }, 'beta-blog')).toBe(false);
  });
});

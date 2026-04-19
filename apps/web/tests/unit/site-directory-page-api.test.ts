import { describe, expect, it } from 'vitest';

import {
  resolveSiteDirectoryAccessSummary,
  resolveSiteDirectorySortSummary,
} from '@/components/site/site-directory-page.api';

describe('site directory page api helpers', () => {
  it('returns default label when sort is empty', () => {
    expect(resolveSiteDirectorySortSummary(null)).toBe('默认排序');
  });

  it('maps access filters to localized summary labels', () => {
    expect(resolveSiteDirectoryAccessSummary('全球')).toBe('全球可访问');
    expect(resolveSiteDirectoryAccessSummary('大陆')).toBe('仅中国大陆可访问');
    expect(resolveSiteDirectoryAccessSummary('海外')).toBe('仅海外可访问');
    expect(resolveSiteDirectoryAccessSummary('unknown')).toBe('');
  });
});

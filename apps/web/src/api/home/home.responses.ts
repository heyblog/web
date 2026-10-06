import { isDate, isPublicAnnouncement, isRecord } from '../announcements/announcements.types.ts';
import { isSiteMetricCounts } from '../site-metrics/site-metrics.types.ts';
import type { HomeSiteCard, HomeSiteTopic } from '../sites/site-card.types.ts';

import type { HomeView } from './home.types.ts';

function isTopic(value: unknown): value is HomeSiteTopic {
  return isRecord(value) && typeof value.name === 'string' && typeof value.slug === 'string';
}

function isSite(value: unknown): value is HomeSiteCard {
  return (
    isRecord(value) &&
    isSiteMetricCounts(value.metrics) &&
    typeof value.shortId === 'string' &&
    (value.customId === null || typeof value.customId === 'string') &&
    typeof value.name === 'string' &&
    typeof value.summary === 'string' &&
    typeof value.host === 'string' &&
    typeof value.homepageUrl === 'string' &&
    (value.accessScope === 'ALL' ||
      value.accessScope === 'CN_ONLY' ||
      value.accessScope === 'GLOBAL_ONLY') &&
    (value.directoryStatus === 'normal' || value.directoryStatus === 'abnormal') &&
    isDate(value.joinedAt) &&
    isDate(value.updatedAt) &&
    (value.classification === null ||
      (isRecord(value.classification) &&
        isTopic(value.classification.level1) &&
        isTopic(value.classification.level2))) &&
    Array.isArray(value.tertiaryTags) &&
    value.tertiaryTags.every(isTopic) &&
    Array.isArray(value.warnings) &&
    value.warnings.every(
      (warning: unknown) =>
        isTopic(warning) && isRecord(warning) && typeof warning.description === 'string',
    ) &&
    (value.defaultFeed === null ||
      (isRecord(value.defaultFeed) &&
        typeof value.defaultFeed.name === 'string' &&
        typeof value.defaultFeed.url === 'string' &&
        (value.defaultFeed.format === 'UNKNOWN' ||
          value.defaultFeed.format === 'RSS' ||
          value.defaultFeed.format === 'ATOM' ||
          value.defaultFeed.format === 'JSON'))) &&
    (value.sitemapUrl === null || typeof value.sitemapUrl === 'string')
  );
}

export function parseHomeView(value: unknown): HomeView | null {
  return isRecord(value) &&
    typeof value.siteCount === 'number' &&
    Number.isSafeInteger(value.siteCount) &&
    value.siteCount >= 0 &&
    Array.isArray(value.announcements) &&
    value.announcements.every(isPublicAnnouncement) &&
    Array.isArray(value.sites) &&
    value.sites.every(isSite)
    ? { siteCount: value.siteCount, announcements: value.announcements, sites: value.sites }
    : null;
}

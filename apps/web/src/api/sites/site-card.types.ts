import type { SiteMetricCounts } from '../site-metrics/site-metrics.types.ts';

export interface SiteCardBase {
  shortId: string;
  customId: string | null;
  name: string;
  summary: string;
  host: string;
  homepageUrl: string;
  accessScope: 'CN_ONLY' | 'GLOBAL_ONLY' | 'ALL';
  directoryStatus: 'normal' | 'abnormal';
  joinedAt: string;
  updatedAt: string;
  metrics: SiteMetricCounts;
}

export interface SiteCardView extends SiteCardBase {
  classification: HomeSiteClassification | null;
  tertiaryTags: HomeSiteTopic[];
  warnings: HomeSiteWarning[];
  defaultFeed: HomeSiteFeed | null;
  sitemapUrl: string | null;
}

export type HomeSiteCard = SiteCardView;

// TODO(home-card): restore articleCount, the content-last-updated marker and feedback
// action when authoritative APIs exist.

export interface HomeSiteTopic {
  name: string;
  slug: string;
}

export interface HomeSiteClassification {
  level1: HomeSiteTopic;
  level2: HomeSiteTopic;
}

export interface HomeSiteWarning {
  name: string;
  slug: string;
  description: string;
}

export interface HomeSiteFeed {
  name: string;
  url: string;
  format: 'UNKNOWN' | 'RSS' | 'ATOM' | 'JSON';
}

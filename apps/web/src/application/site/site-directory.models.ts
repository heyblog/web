export interface SiteWarningTagView {
  machineKey: string;
  name: string;
  description: string | null;
}

export interface SiteDirectoryItem {
  id: string;
  bid: string | null;
  slug: string;
  name: string;
  url: string;
  sign: string;
  feedUrl: string | null;
  sitemap: string | null;
  linkPage: string | null;
  featured: boolean;
  status: string;
  accessScope: string;
  joinTime: string;
  updateTime: string;
  latestPublishedTime: string | null;
  articleCount: number;
  visitCount: number;
  primaryTag: string | null;
  subTags: string[];
  warningTags: SiteWarningTagView[];
}

export interface SiteDirectoryMeta {
  stats: {
    totalSites: number;
    normalSites: number;
    abnormalSites: number;
    rssSites: number;
  };
  filters: {
    mainTags: Array<{ id: string; name: string }>;
    subTags: Array<{ id: string; name: string }>;
    warningTags: Array<{ id: string; machineKey: string | null; name: string }>;
    programs: Array<{ id: string; name: string }>;
  };
  defaults: {
    pageSize: number;
    random: boolean;
    statusMode: 'normal' | 'abnormal';
  };
}

export interface SiteDirectoryResult {
  items: SiteDirectoryItem[];
  pagination: {
    page: number;
    pageSize: number;
    totalItems: number;
    totalPages: number;
  };
  query: {
    q: string;
    main: string[];
    sub: string[];
    warning: string[];
    program: string[];
    statusMode: 'normal' | 'abnormal';
    random: boolean;
    sort: 'updated' | 'joined' | 'visits' | 'articles' | null;
    order: 'asc' | 'desc';
    randomSeed: string;
  };
}

export type SiteGoFailureReason =
  | 'UNKNOWN_PARAM'
  | 'INVALID_PARAMS'
  | 'INVALID_RECOMMEND'
  | 'INVALID_TYPE'
  | 'DUPLICATE_PARAM'
  | 'NO_MATCH';

export interface SiteGoFilters {
  recommend: boolean;
  type: string;
}

export interface SiteGoResult {
  site: SiteDirectoryItem | null;
  availableTypes: string[];
  filters: SiteGoFilters;
  failureReason: SiteGoFailureReason | null;
}

export interface SiteDetail extends SiteDirectoryItem {
  reason: string | null;
  feeds: Array<{
    name: string | null;
    url: string;
    type: string | null;
    isDefault: boolean;
  }>;
  architecture: {
    program: {
      id: string;
      name: string;
      isOpenSource: boolean;
      websiteUrl: string | null;
      repoUrl: string | null;
    } | null;
  };
  heartbeatChecks: SiteCheckItem[];
}

export interface SiteArticleItem {
  id: string;
  title: string;
  articleUrl: string;
  summary: string | null;
  publishedTime: string | null;
  fetchedTime: string;
  source: {
    feedName: string | null;
    feedUrl: string | null;
    feedType: string | null;
  };
}

export interface SiteCheckItem {
  id: string;
  region: string;
  result: string;
  statusCode: number | null;
  responseTimeMs: number | null;
  durationMs: number | null;
  message: string | null;
  finalUrl: string | null;
  contentVerified: boolean;
  checkTime: string;
}

export interface PagedResult<TItem> {
  items: TItem[];
  pagination: {
    page: number;
    pageSize: number;
    totalItems: number;
    totalPages: number;
  };
}

export interface SiteDirectoryPreference {
  randomMode: 'stable' | 'off';
  randomSeed: string | null;
}

export interface SiteFeedbackPayload {
  reasonType: string;
  feedbackContent: string;
  reporterName?: string | null;
  reporterEmail?: string | null;
  notifyByEmail?: boolean;
}

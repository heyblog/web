import type { SiteFeedbackReason, SiteWarningTagMachineKey } from '@zhblogs/db';

export type PublicSiteTagState = {
  primaryTag: string | null;
  subTags: string[];
};

export type PublicSiteWarningTag = {
  machineKey: SiteWarningTagMachineKey;
  name: string;
  description: string | null;
};

export type PublicSiteDirectoryItem = {
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
  warningTags: PublicSiteWarningTag[];
};

export type PublicSiteDirectoryMeta = {
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
};

export type PublicSiteDirectoryQuery = {
  q?: string;
  main?: string[];
  sub?: string[];
  warning?: string[];
  program?: string[];
  statusMode?: 'normal' | 'abnormal';
  random?: boolean;
  sort?: 'updated' | 'joined' | 'visits' | 'articles';
  order?: 'asc' | 'desc';
  page?: number;
  pageSize?: number;
  randomSeed?: string;
};

export type PublicSiteDirectoryResult = {
  items: PublicSiteDirectoryItem[];
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
};

export type PublicSiteRandomFailureReason =
  | 'UNKNOWN_PARAM'
  | 'INVALID_PARAMS'
  | 'INVALID_RECOMMEND'
  | 'INVALID_TYPE'
  | 'DUPLICATE_PARAM'
  | 'NO_MATCH';

export type PublicSiteRandomFilters = {
  recommend: boolean;
  type: string;
};

export type PublicSiteRandomResult = {
  site: PublicSiteDirectoryItem | null;
  availableTypes: string[];
  filters: PublicSiteRandomFilters;
  failureReason: PublicSiteRandomFailureReason | null;
};

export type PublicSiteDetail = PublicSiteDirectoryItem & {
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
  heartbeatChecks: PublicSiteCheckItem[];
};

export type PublicSiteArticleItem = {
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
};

export type PublicSiteCheckItem = {
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
};

export type PublicSiteDetailTabPage<TItem> = {
  items: TItem[];
  pagination: {
    page: number;
    pageSize: number;
    totalItems: number;
    totalPages: number;
  };
};

export type PublicSiteSubscriptionSummary = {
  todayArticles: number;
  weekArticles: number;
  totalArticles: number;
  siteCount: number;
};

export type PublicSiteSubscriptionItem = PublicSiteArticleItem & {
  site: {
    id: string;
    slug: string;
    name: string;
    url: string;
  };
};

export type PublicSiteSubscriptionResult = {
  summary: PublicSiteSubscriptionSummary;
  items: PublicSiteSubscriptionItem[];
  pagination: {
    page: number;
    pageSize: number;
    totalItems: number;
    totalPages: number;
  };
};

export type PublicSiteFeedbackInput = {
  slug: string;
  reasonType: SiteFeedbackReason;
  feedbackContent: string;
  reporterName?: string | null;
  reporterEmail?: string | null;
  notifyByEmail?: boolean;
};

export type SiteDirectoryPreference = {
  randomMode: 'stable' | 'off';
  randomSeed: string | null;
};

export type PublicSiteAccessSource = 'SITE_GO' | 'SITE_DETAIL' | 'SITE_CARD';

export type PublicSiteAccessTargetKind = 'SITE' | 'FEED' | 'SITEMAP' | 'LINK_PAGE' | 'ARTICLE';

export type PublicSiteAccessEventPayload = {
  source: PublicSiteAccessSource;
  targetKind: PublicSiteAccessTargetKind;
  path: string;
};

export type PublicSiteAccessEventInput = PublicSiteAccessEventPayload & {
  id: string;
  referer: string | null;
  origin: string | null;
  userAgent: string | null;
};

export type PublicSiteAccessEventResult = {
  recorded: boolean;
};

export type PublicSiteBaseRow = {
  id: string;
  bid: string | null;
  name: string;
  url: string;
  sign: string | null;
  feeds: Array<{
    url?: string | null;
    isDefault?: boolean | null;
  }>;
  sitemap: string | null;
  linkPage: string | null;
  featured: boolean;
  status: string;
  accessScope: string;
  joinTime: Date;
  updateTime: Date;
  reason: string | null;
};

export type StructuredQueryState = {
  keywords: string[];
  main: string[];
  sub: string[];
  warning: string[];
  program: string[];
  site: string[];
  domain: string[];
  access: string[];
  rss: boolean | null;
  featured: boolean | null;
};

export type DirectoryState = Omit<
  PublicSiteDirectoryQuery,
  | 'main'
  | 'sub'
  | 'warning'
  | 'program'
  | 'q'
  | 'page'
  | 'pageSize'
  | 'random'
  | 'randomSeed'
  | 'statusMode'
  | 'sort'
  | 'order'
> & {
  main: string[];
  sub: string[];
  warning: string[];
  program: string[];
  q: string;
  page: number;
  pageSize: number;
  random: boolean;
  randomSeed: string;
  statusMode: 'normal' | 'abnormal';
  sort: 'updated' | 'joined' | 'visits' | 'articles' | null;
  order: 'asc' | 'desc';
  keywords: string[];
  site: string[];
  domain: string[];
  access: string[];
  rss: boolean | null;
  featured: boolean | null;
};

export type StructuredArrayField =
  | 'main'
  | 'sub'
  | 'warning'
  | 'program'
  | 'site'
  | 'domain'
  | 'access';

export type SiteArchitectureFilterState = {
  programBySiteId: Map<string, string>;
  filters: {
    programs: Array<{ id: string; name: string }>;
  };
};

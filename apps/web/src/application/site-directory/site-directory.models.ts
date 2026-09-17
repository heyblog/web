import type { SiteCardView } from '@/application/home/home.shared';

export type SiteDirectoryFeed = 'any' | 'with' | 'without';
export type SiteDirectorySort = 'random' | 'joined' | 'updated';
export type SiteDirectoryOrder = 'asc' | 'desc';
export type SiteDirectoryAccess = 'ALL' | 'CN_ONLY' | 'GLOBAL_ONLY';
export type SiteDirectoryStatus = 'normal' | 'abnormal';
export type SiteDirectoryFilterName = 'tertiary' | 'warning' | 'technology' | 'access';

export type SiteDirectoryQuery = {
  readonly page: number;
  readonly q: string;
  readonly level1: string;
  readonly level2: string;
  readonly tertiary: readonly string[];
  readonly warning: readonly string[];
  readonly technology: readonly string[];
  readonly access: readonly SiteDirectoryAccess[];
  readonly feed: SiteDirectoryFeed;
  readonly status: SiteDirectoryStatus;
  readonly sort: SiteDirectorySort;
  readonly order: SiteDirectoryOrder;
  readonly seed: string;
};

export type SiteDirectoryPagination = {
  readonly page: number;
  readonly pageSize: number;
  readonly totalItems: number;
  readonly totalPages: number;
};

export type SiteDirectoryView = {
  readonly items: readonly SiteCardView[];
  readonly pagination: SiteDirectoryPagination;
  readonly query: SiteDirectoryQuery;
  readonly statusCounts: Readonly<Record<SiteDirectoryStatus, number>>;
};

export type SiteDirectoryOption = {
  readonly value: string;
  readonly label: string;
  readonly normalCount: number;
  readonly abnormalCount: number;
};

export type SiteDirectoryClassificationOption = SiteDirectoryOption & {
  readonly children: readonly SiteDirectoryOption[];
};

export type SiteDirectoryOptions = {
  readonly classifications: readonly SiteDirectoryClassificationOption[];
  readonly tertiaryTags: readonly SiteDirectoryOption[];
  readonly warnings: readonly SiteDirectoryOption[];
  readonly technologies: readonly SiteDirectoryOption[];
};

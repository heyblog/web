export interface SiteSubscriptionSummary {
  todayArticles: number;
  weekArticles: number;
  totalArticles: number;
  siteCount: number;
}

export interface SiteSubscriptionItem {
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
  site: {
    id: string;
    slug: string;
    name: string;
    url: string;
  };
}

export interface SiteSubscriptionResult {
  summary: SiteSubscriptionSummary;
  items: SiteSubscriptionItem[];
  pagination: {
    page: number;
    pageSize: number;
    totalItems: number;
    totalPages: number;
  };
}

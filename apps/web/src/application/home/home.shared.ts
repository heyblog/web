export interface HomeAnnouncementAction {
  label: string;
  href: string;
  external: boolean;
}

export interface HomeAnnouncement {
  title: string;
  startsAt: string;
  action: HomeAnnouncementAction | null;
}

export interface SiteCardBase {
  shortId: string;
  customId: string | null;
  name: string;
  summary: string;
  host: string;
  homepageUrl: string;
  accessScope: 'CN_ONLY' | 'GLOBAL_ONLY' | 'ALL';
  joinedAt: string;
  updatedAt: string;
}

export interface HomeSiteCard extends SiteCardBase {
  topics: HomeSiteTopic[];
  warnings: HomeSiteWarning[];
  defaultFeed: HomeSiteFeed | null;
  sitemapUrl: string | null;
}

// TODO(home-card): restore visitCount, articleCount, the content-last-updated marker, old
// tone/color logic, feedback action, and legacy UUID tracking when authoritative APIs exist.

export interface HomeSiteTopic {
  name: string;
  slug: string;
  role: 'PRIMARY' | 'SECONDARY';
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

export interface HomeView {
  siteCount: number;
  announcement: HomeAnnouncement | null;
  sites: HomeSiteCard[];
}

export type HomeMockMode = 'cards' | 'empty' | 'error';

interface SiteIdentifier {
  shortId: string;
}

export function siteDetailPath(site: SiteIdentifier): string {
  return `/site/${encodeURIComponent(site.shortId)}`;
}

export function formatSiteJoinedAt(value: string): string {
  const date = new Date(value);

  return Number.isNaN(date.getTime())
    ? '已加入目录'
    : `${date.toLocaleDateString('zh-CN', {
        year: 'numeric',
        month: 'short',
        timeZone: 'UTC',
      })}加入`;
}

export function formatSiteUpdatedAt(value: string): string {
  const date = new Date(value);

  return Number.isNaN(date.getTime())
    ? '信息更新时间未知'
    : `${date.toLocaleDateString('zh-CN', {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
        timeZone: 'UTC',
      })}更新`;
}

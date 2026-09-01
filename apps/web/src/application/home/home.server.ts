import { type ApiJsonResult, fetchApiJson } from '@/application/api/client.server';

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

export interface HomeSiteCard {
  shortId: string;
  customId: string | null;
  name: string;
  summary: string;
  host: string;
  homepageUrl: string;
  accessScope: 'CN_ONLY' | 'GLOBAL_ONLY' | 'ALL';
  joinedAt: string;
}

export interface HomeView {
  siteCount: number;
  announcement: HomeAnnouncement | null;
  sites: HomeSiteCard[];
}

export function loadHome(signal?: AbortSignal): Promise<ApiJsonResult<HomeView>> {
  return fetchApiJson<HomeView>('/home', { signal });
}

export function siteDetailPath(site: Pick<HomeSiteCard, 'customId' | 'shortId'>): string {
  return site.customId
    ? `/s/${encodeURIComponent(site.customId)}`
    : `/site/${encodeURIComponent(site.shortId)}`;
}

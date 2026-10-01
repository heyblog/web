import { type HomeSiteCard } from '../sites/site-card.types.ts';

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

export interface HomeView {
  siteCount: number;
  announcement: HomeAnnouncement | null;
  sites: HomeSiteCard[];
}

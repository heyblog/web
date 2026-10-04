import type { PublicAnnouncement } from '../announcements/announcements.types.ts';
import { type HomeSiteCard } from '../sites/site-card.types.ts';

export type HomeAnnouncement = PublicAnnouncement;

export interface HomeView {
  siteCount: number;
  announcements: readonly HomeAnnouncement[];
  sites: HomeSiteCard[];
}

import { type SiteCardBase } from './site-card.types.ts';

export interface SiteTopic {
  name: string;
  slug: string;
  description: string;
}

export interface SiteClassification {
  level1: SiteTopic;
  level2: SiteTopic;
}

export interface SiteWarning {
  name: string;
  slug: string;
  description: string;
}

export interface SiteFeed {
  name: string;
  url: string;
  format: 'UNKNOWN' | 'RSS' | 'ATOM' | 'JSON';
  isDefault: boolean;
}

export interface SiteResource {
  kind: 'SITEMAP' | 'LINK_PAGE';
  url: string;
}

export interface SiteTechnology {
  name: string;
  role: 'SITE_PROGRAM' | 'FRAMEWORK' | 'LANGUAGE' | 'RUNTIME' | 'OTHER';
  homepageUrl: string | null;
  repositoryUrl: string | null;
  isOpenSource: boolean;
}

export interface SiteProfile extends SiteCardBase {
  readonly iconHash: string | null;
  classification: SiteClassification | null;
  tertiaryTags: SiteTopic[];
  warnings: SiteWarning[];
  feeds: SiteFeed[];
  resources: SiteResource[];
  technologies: SiteTechnology[];
}

export interface SiteLink {
  label: string;
  href: string;
  external?: boolean;
}

export interface SiteNavigationItem extends SiteLink {
  match: 'exact' | 'prefix';
}

export interface SiteConfig {
  name: string;
  title: string;
  description: string;
  url: string;
  language: string;
  locale: string;
  robots: string;
  themeStorageKey: string;
  openGraph: {
    type: 'website' | 'article';
    imagePath: string;
    imageAlt: string;
  };
  twitterCard: 'summary' | 'summary_large_image';
  navigation: readonly SiteNavigationItem[];
  footer: {
    copyrightStartYear: number;
    copyrightOwner: string;
    registration: SiteLink;
    statementLinks: readonly SiteLink[];
    friendLinks: readonly SiteLink[];
  };
}

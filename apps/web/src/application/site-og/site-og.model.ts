import type { SiteProfile } from '../../api/sites/site-profile.types.ts';
import { siteConfig } from '../../site.config.ts';

export interface SiteOgContent {
  readonly detailUrl: string;
  readonly iconHash: string | null;
  readonly iconDataUrl?: string;
  readonly name: string;
  readonly description: string;
  readonly host: string;
  readonly classification: readonly [string, string] | null;
}

export function siteOgContent(profile: SiteProfile): SiteOgContent {
  return {
    detailUrl: new URL(`/site/${encodeURIComponent(profile.shortId)}`, siteConfig.url).href,
    iconHash: profile.iconHash,
    name: profile.name.trim(),
    description:
      profile.summary.trim() || `${profile.name.trim()}（${profile.host}）的博客收录信息。`,
    host: profile.host,
    classification: profile.classification
      ? [profile.classification.level1.name, profile.classification.level2.name]
      : null,
  };
}

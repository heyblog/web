import { createHash } from 'node:crypto';

import { pageTitleWithBrand } from '../../shared/seo.ts';
import { siteConfig } from '../../site.config.ts';
import type { SiteProfile } from '../site-profile/site-profile.server.ts';

export interface SiteOgContent {
  readonly detailUrl: string;
  readonly iconHash: string | null;
  readonly iconDataUrl?: string;
  readonly name: string;
  readonly description: string;
  readonly host: string;
  readonly classification: readonly [string, string] | null;
}

// Bump whenever the card's layout, fonts, or rendering behavior changes.
const templateVersion = '6';

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

export function siteOgImagePath(profile: SiteProfile): string {
  const version = createHash('sha256')
    .update(JSON.stringify([templateVersion, siteOgContent(profile)]))
    .digest('hex')
    .slice(0, 16);
  return `/og/site/${encodeURIComponent(profile.shortId)}.png?v=${version}`;
}

export function siteProfileMetadata(profile: SiteProfile) {
  const content = siteOgContent(profile);
  return {
    title: content.name,
    siteName: pageTitleWithBrand(content.name),
    description: content.description,
    imagePath: siteOgImagePath(profile),
    imageAlt: `${content.name}的博客分享卡片`,
    imageType: 'image/png',
    imageWidth: 1200,
    imageHeight: 630,
  };
}

import { createHash } from 'node:crypto';

import type { SiteProfile } from '../../api/sites/site-profile.types.ts';
import { siteConfig } from '../../site.config.ts';

import { siteOgContent } from './site-og.model.ts';

// Bump whenever the card's layout, fonts, or rendering behavior changes.
const templateVersion = '6';

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
    siteName: siteConfig.name,
    description: content.description,
    imagePath: siteOgImagePath(profile),
    imageAlt: `${content.name}的博客分享卡片`,
    imageType: 'image/png',
    imageWidth: 1200,
    imageHeight: 630,
  };
}

import { siteConfig } from '../site.config.ts';

import { isIndexablePath, listingSeo } from './indexing.ts';

export interface PageMetadataInput {
  pathname: string;
  search?: string;
  status?: number;
  title?: string;
  siteName?: string;
  description?: string;
  canonicalPath?: string;
  ogType?: 'website' | 'article';
  imagePath?: string;
  imageAlt?: string;
  imageType?: string;
  imageWidth?: number;
  imageHeight?: number;
  robots?: string;
  publishedTime?: string | Date;
  modifiedTime?: string | Date;
}

export function toIsoDate(value: string | Date | undefined): string | undefined {
  if (!value) {
    return undefined;
  }

  const date = value instanceof Date ? value : new Date(value);

  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}

export function pageTitleWithBrand(value: string): string {
  const title = value.trim();
  const suffix = ` | ${siteConfig.name}`;
  return title.endsWith(suffix) ? title : `${title}${suffix}`;
}

export function resolvePageMetadata(input: PageMetadataInput) {
  const pageTitle = input.title?.trim();
  const title = pageTitle ? pageTitleWithBrand(pageTitle) : siteConfig.title;
  const listing = listingSeo(input.pathname, input.search);
  const canonical = new URL(
    listing.isListing ? listing.canonicalPath : (input.canonicalPath ?? listing.canonicalPath),
    siteConfig.url,
  );
  canonical.pathname = canonical.pathname.replace(/\/+$/, '') || '/';
  canonical.hash = '';
  const canonicalUrl = canonical.toString();
  const indexable =
    isIndexablePath(input.pathname) && listing.indexable && (input.status ?? 200) < 400;
  const imageUrl = new URL(
    input.imagePath ?? siteConfig.openGraph.imagePath,
    siteConfig.url,
  ).toString();

  return {
    title,
    siteName: input.siteName?.trim() || siteConfig.name,
    description: input.description?.trim() || siteConfig.description,
    robots: indexable ? input.robots?.trim() || siteConfig.robots : 'noindex, follow',
    canonicalUrl,
    imageUrl,
    imageAlt: input.imageAlt?.trim() || siteConfig.openGraph.imageAlt,
    imageType: input.imageType ?? (input.imagePath ? undefined : 'image/png'),
    imageWidth: input.imageWidth ?? (input.imagePath ? undefined : 1200),
    imageHeight: input.imageHeight ?? (input.imagePath ? undefined : 630),
    ogType: input.ogType ?? siteConfig.openGraph.type,
    publishedTime: toIsoDate(input.publishedTime),
    modifiedTime: toIsoDate(input.modifiedTime),
  };
}

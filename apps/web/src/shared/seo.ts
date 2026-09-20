import { siteConfig } from '../site.config.ts';

export interface PageMetadataInput {
  pathname: string;
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

function toIsoDate(value: string | Date | undefined): string | undefined {
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
  const canonicalUrl = new URL(input.canonicalPath ?? input.pathname, siteConfig.url).toString();
  const imageUrl = new URL(
    input.imagePath ?? siteConfig.openGraph.imagePath,
    siteConfig.url,
  ).toString();

  return {
    title,
    siteName: input.siteName?.trim() || siteConfig.name,
    description: input.description?.trim() || siteConfig.description,
    robots: input.robots?.trim() || siteConfig.robots,
    canonicalUrl,
    imageUrl,
    imageAlt: input.imageAlt?.trim() || siteConfig.openGraph.imageAlt,
    imageType: input.imageType ?? (input.imagePath ? undefined : 'image/svg+xml'),
    imageWidth: input.imageWidth ?? (input.imagePath ? undefined : 1200),
    imageHeight: input.imageHeight ?? (input.imagePath ? undefined : 630),
    ogType: input.ogType ?? siteConfig.openGraph.type,
    publishedTime: toIsoDate(input.publishedTime),
    modifiedTime: toIsoDate(input.modifiedTime),
  };
}

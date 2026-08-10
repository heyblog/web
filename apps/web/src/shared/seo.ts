import { siteConfig } from '@/site.config';

export interface PageMetadataInput {
  pathname: string;
  title?: string;
  description?: string;
  canonicalPath?: string;
  ogType?: 'website' | 'article';
  imagePath?: string;
  imageAlt?: string;
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

export function resolvePageMetadata(input: PageMetadataInput) {
  const pageTitle = input.title?.trim();
  const title =
    pageTitle && pageTitle !== siteConfig.name
      ? `${pageTitle} | ${siteConfig.name}`
      : siteConfig.title;
  const canonicalUrl = new URL(input.canonicalPath ?? input.pathname, siteConfig.url).toString();
  const imageUrl = new URL(
    input.imagePath ?? siteConfig.openGraph.imagePath,
    siteConfig.url,
  ).toString();

  return {
    title,
    description: input.description?.trim() || siteConfig.description,
    robots: input.robots?.trim() || siteConfig.robots,
    canonicalUrl,
    imageUrl,
    imageAlt: input.imageAlt?.trim() || siteConfig.openGraph.imageAlt,
    ogType: input.ogType ?? siteConfig.openGraph.type,
    publishedTime: toIsoDate(input.publishedTime),
    modifiedTime: toIsoDate(input.modifiedTime),
  };
}

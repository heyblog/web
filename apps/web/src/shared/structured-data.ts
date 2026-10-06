import { siteConfig } from '../site.config.ts';

import { toIsoDate } from './seo.ts';

type JsonValue = string | number | boolean | null | readonly JsonValue[] | StructuredData;
export interface StructuredData {
  readonly [key: string]: JsonValue | undefined;
}
export interface Breadcrumb {
  readonly name: string;
  readonly path: string;
}
const absolute = (path: string) => new URL(path, siteConfig.url).href;

export function siteStructuredData(): StructuredData[] {
  return [
    {
      '@type': 'Organization',
      '@id': absolute('/#publisher'),
      name: siteConfig.name,
      url: siteConfig.url,
      logo: absolute('/favicon.png'),
    },
    {
      '@type': 'WebSite',
      '@id': absolute('/#website'),
      name: siteConfig.name,
      url: siteConfig.url,
      description: siteConfig.description,
      inLanguage: siteConfig.language,
      publisher: { '@id': absolute('/#publisher') },
    },
  ];
}

export function breadcrumbStructuredData(items: readonly Breadcrumb[]): StructuredData {
  return {
    '@type': 'BreadcrumbList',
    itemListElement: items.map((item, index) => ({
      '@type': 'ListItem',
      position: index + 1,
      name: item.name,
      item: absolute(item.path),
    })),
  };
}

export interface ArticleMetadata {
  readonly title: string;
  readonly description: string;
  readonly path: string;
  readonly publishedTime?: string | Date;
  readonly modifiedTime?: string | Date;
  readonly authors?: readonly { readonly label: string; readonly href: string }[];
}

export function articleStructuredData(input: ArticleMetadata): StructuredData {
  const published = toIsoDate(input.publishedTime);
  const modified = toIsoDate(input.modifiedTime);
  return {
    '@type': 'Article',
    '@id': absolute(`${input.path}#article`),
    headline: input.title,
    description: input.description,
    mainEntityOfPage: absolute(input.path),
    image: absolute(siteConfig.openGraph.imagePath),
    inLanguage: siteConfig.language,
    publisher: { '@id': absolute('/#publisher') },
    author: input.authors?.map((author) => ({
      '@type': 'Person',
      name: author.label,
      url: absolute(author.href),
    })),
    datePublished: published,
    dateModified: modified && (!published || modified >= published) ? modified : undefined,
  };
}

export function serializeStructuredData(graph: readonly StructuredData[]): string {
  return JSON.stringify({ '@context': 'https://schema.org', '@graph': graph }).replace(
    /[<>&\u2028\u2029]/g,
    (character) => `\\u${character.charCodeAt(0).toString(16).padStart(4, '0')}`,
  );
}

import type {
  SiteDirectoryItem,
  SiteWarningTagView,
} from '@/application/site/site-directory.models';

export type BlogCardTone = 'amber' | 'blue' | 'emerald' | 'red' | 'stone';
export type BlogCardUpdatedTone = 'amber' | 'blue' | 'emerald' | 'stone';
type BlogCardUpdatedMeta = {
  label: string;
  tone: BlogCardUpdatedTone;
};

export interface SiteCardEntry {
  id: string;
  slug: string;
  name: string;
  domain: string;
  href: string;
  shortCode: string;
  primaryTag: string;
  summary: string;
  subTags: string[];
  warningTags: SiteWarningTagView[];
  joinedAt: string;
  joinedLabel: string;
  updatedLabel: string | null;
  updatedTone: BlogCardUpdatedTone | null;
  articleCount?: string;
  visitCount: string;
  tone: BlogCardTone;
  rssUrl?: string;
  sitemapUrl?: string;
  featured?: boolean;
}

export function extractDomain(url: string): string {
  try {
    const target = new URL(url);
    return `${target.host}${target.pathname === '/' ? '' : target.pathname}`;
  } catch {
    return url;
  }
}

export function createShortCode(name: string): string {
  const compact = name.replace(/\s+/g, '');
  const chineseChars = [...compact].filter((char) => /[\u4e00-\u9fa5]/.test(char));

  if (chineseChars.length >= 2) {
    return `${chineseChars[0]}${chineseChars[1]}`;
  }

  const letters = name
    .split(/[\s/|·_.-]+/)
    .map((token) => token.trim().charAt(0))
    .filter(Boolean)
    .slice(0, 2)
    .join('')
    .toUpperCase();

  return letters || compact.slice(0, 2).toUpperCase() || 'SB';
}

export function formatYearMonth(value: string): string {
  const target = new Date(value);

  return Number.isNaN(target.getTime())
    ? value
    : `${target.getFullYear()}.${String(target.getMonth() + 1).padStart(2, '0')}`;
}

export function formatCompactCount(value: number): string {
  if (value >= 1000) {
    return `${(value / 1000).toFixed(value >= 10000 ? 0 : 1).replace(/\.0$/, '')}k`;
  }

  return String(value);
}

export function resolveUpdatedLabel(value: string | null): string | null {
  return resolveUpdatedMeta(value)?.label ?? null;
}

export function resolveTone(primaryTag: string, featured: boolean): BlogCardTone {
  if (featured) {
    return 'red';
  }

  if (/(技术|编程|开发|后端|前端|架构)/.test(primaryTag)) {
    return 'blue';
  }

  if (/(设计|产品|人文|阅读|写作)/.test(primaryTag)) {
    return 'amber';
  }

  if (/(生活|摄影|旅行|社区|外联)/.test(primaryTag)) {
    return 'emerald';
  }

  return 'stone';
}

export function resolveUpdatedTone(latestPublishedTime: string | null): BlogCardUpdatedTone | null {
  return resolveUpdatedMeta(latestPublishedTime)?.tone ?? null;
}

export function mapDirectoryItemToSiteCardEntry(item: SiteDirectoryItem): SiteCardEntry {
  const primaryTag = item.primaryTag ?? '未分类';
  const updatedMeta = item.feedUrl ? resolveUpdatedMeta(item.latestPublishedTime) : null;

  return {
    id: item.id,
    slug: item.slug,
    name: item.name,
    domain: extractDomain(item.url),
    href: item.url,
    shortCode: createShortCode(item.name),
    primaryTag,
    summary: item.sign || `${primaryTag}方向的公开站点。`,
    subTags: item.subTags,
    warningTags: item.warningTags,
    joinedAt: item.joinTime,
    joinedLabel: formatYearMonth(item.joinTime),
    updatedLabel: updatedMeta?.label ?? null,
    updatedTone: updatedMeta?.tone ?? null,
    articleCount: item.articleCount > 0 ? String(item.articleCount) : undefined,
    visitCount: formatCompactCount(item.visitCount),
    tone: resolveTone(primaryTag, item.featured),
    rssUrl: item.feedUrl ?? undefined,
    sitemapUrl: item.sitemap ?? undefined,
    featured: item.featured,
  };
}

function resolveUpdatedMeta(value: string | null): BlogCardUpdatedMeta | null {
  const diffDays = resolveUpdatedDiffDays(value);
  if (diffDays === null) {
    return null;
  }

  return {
    label: formatUpdatedLabel(diffDays),
    tone: resolveUpdatedToneByDiffDays(diffDays),
  };
}

function resolveUpdatedDiffDays(value: string | null): number | null {
  if (!value) {
    return null;
  }

  const publishedTime = new Date(value).getTime();
  if (Number.isNaN(publishedTime)) {
    return null;
  }

  const diffMs = Date.now() - publishedTime;
  if (diffMs < -36 * 60 * 60 * 1000) {
    return null;
  }

  return Math.floor(Math.max(diffMs, 0) / 86_400_000);
}

function formatUpdatedLabel(diffDays: number): string {
  if (diffDays === 0) {
    return '今天更新';
  }

  if (diffDays < 7) {
    return `${diffDays} 天前更新`;
  }

  if (diffDays < 30) {
    return `${Math.floor(diffDays / 7)} 周前更新`;
  }

  if (diffDays < 365) {
    return `${Math.floor(diffDays / 30)} 个月前更新`;
  }

  return `${Math.floor(diffDays / 365)} 年前更新`;
}

function resolveUpdatedToneByDiffDays(diffDays: number): BlogCardUpdatedTone {
  if (diffDays < 7) {
    return 'emerald';
  }

  if (diffDays < 31) {
    return 'amber';
  }

  if (diffDays < 183) {
    return 'blue';
  }

  return 'stone';
}

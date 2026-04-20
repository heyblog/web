import { fetchApiJson } from '../api/api.server';

import {
  createShortCode,
  extractDomain,
  formatCompactCount,
  formatYearMonth,
  resolveTone,
  resolveUpdatedLabel,
  resolveUpdatedTone,
  type SiteCardEntry,
} from './site-card.shared';

interface ApiPublicSiteItem {
  id: string;
  bid: string | null;
  name: string;
  url: string;
  sign: string;
  feedUrl: string | null;
  sitemap: string | null;
  linkPage: string | null;
  featured: boolean;
  status: string;
  accessScope: string;
  joinTime: string;
  updateTime: string;
  latestPublishedTime: string | null;
  articleCount: number;
  visitCount: number;
  primaryTag: string | null;
  subTags: string[];
  warningTags: Array<{
    machineKey: string;
    name: string;
    description: string | null;
  }>;
}

interface PublicSitesPayload {
  ok: boolean;
  data: {
    items: ApiPublicSiteItem[];
  };
}

export interface PublicSiteEntry extends SiteCardEntry {
  id: string;
  bid: string | null;
  profile: string;
  highlights: string[];
  articleCountValue: number;
  visitCountValue: number;
  linkPage?: string;
  accessScope: string;
  siteStatus: string;
  latestPublishedTime: string | null;
  updateTime: string;
}

function createSiteSlug(item: Pick<ApiPublicSiteItem, 'bid' | 'name' | 'id'>): string {
  return item.id;
}

export function matchesSiteSlug(item: Pick<PublicSiteEntry, 'id' | 'slug'>, slug: string): boolean {
  return item.slug === slug || item.id === slug;
}

export function formatAccessScopeLabel(value: string): string {
  if (value === 'CN_ONLY') {
    return '仅中国大陆可访问';
  }

  if (value === 'NON_CN_ONLY') {
    return '仅海外可访问';
  }

  return '全球可访问';
}

export function formatSiteStatusLabel(value: string): string {
  if (value === 'ERROR') {
    return '访问异常';
  }

  if (value === 'WARNING') {
    return '部分异常';
  }

  return '状态正常';
}

function createHighlights(
  item: ApiPublicSiteItem,
  primaryTag: string,
  updatedLabel?: string | null,
): string[] {
  const highlights: string[] = [];

  highlights.push(
    `分类：${primaryTag}${item.subTags.length ? ` · ${item.subTags.slice(0, 2).join(' / ')}` : ''}`,
  );

  if (item.articleCount > 0) {
    highlights.push(`已聚合 ${item.articleCount} 篇公开文章`);
  }

  if (updatedLabel) {
    highlights.push(`最近内容更新：${updatedLabel}`);
  } else {
    highlights.push(`站点状态：${formatSiteStatusLabel(item.status)}`);
  }

  const resources = [
    item.feedUrl ? 'RSS' : null,
    item.sitemap ? '站点地图' : null,
    item.linkPage ? '友链页' : null,
  ].filter(Boolean);

  if (resources.length > 0) {
    highlights.push(`已提供 ${resources.join('、')} 入口`);
  }

  if (item.warningTags.length > 0) {
    highlights.push(
      `警示：${item.warningTags
        .map((tag) => tag.name)
        .slice(0, 2)
        .join(' / ')}`,
    );
  }

  return highlights.slice(0, 3);
}

function mapPublicSite(item: ApiPublicSiteItem): PublicSiteEntry {
  const primaryTag = item.primaryTag ?? '未分类';
  const updatedLabel = item.feedUrl ? resolveUpdatedLabel(item.latestPublishedTime) : null;

  return {
    id: item.id,
    bid: item.bid,
    slug: createSiteSlug(item),
    name: item.name,
    domain: extractDomain(item.url),
    href: item.url,
    shortCode: createShortCode(item.name),
    primaryTag,
    summary: item.sign || `${primaryTag}方向的公开站点。`,
    profile:
      item.sign ||
      `这个站点已收录到公开目录，当前显示为${formatSiteStatusLabel(item.status)}，可继续从下方标签与订阅入口了解它。`,
    highlights: createHighlights(item, primaryTag, updatedLabel),
    subTags: item.subTags,
    warningTags: item.warningTags,
    joinedAt: item.joinTime,
    joinedLabel: formatYearMonth(item.joinTime),
    updatedLabel,
    updatedTone: item.feedUrl ? resolveUpdatedTone(item.latestPublishedTime) : null,
    articleCount: item.articleCount > 0 ? String(item.articleCount) : undefined,
    articleCountValue: item.articleCount,
    visitCount: formatCompactCount(item.visitCount),
    visitCountValue: item.visitCount,
    tone: resolveTone(primaryTag, item.featured),
    rssUrl: item.feedUrl ?? undefined,
    sitemapUrl: item.sitemap ?? undefined,
    linkPage: item.linkPage ?? undefined,
    featured: item.featured,
    accessScope: item.accessScope,
    siteStatus: item.status,
    latestPublishedTime: item.latestPublishedTime,
    updateTime: item.updateTime,
  };
}

export async function readRandomPublicSites(limit = 6): Promise<PublicSiteEntry[]> {
  const payload = await fetchApiJson<PublicSitesPayload>(
    `/api/public/sites?page=1&pageSize=${limit}&random=on`,
  );

  return payload?.data.items.map(mapPublicSite) ?? [];
}

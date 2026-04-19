import type {
  PagedResult,
  SiteCheckItem,
  SiteDetail,
} from '@/application/site/site-directory.models';

export const DEFAULT_SITE_DETAIL_DESCRIPTION =
  '该站点暂未提供站点简介，可继续查看文章聚合、状态检测与相关资源入口。';

export function resolveSiteDetailDescription(sign: string | null | undefined): string {
  const value = sign?.trim();

  return value || DEFAULT_SITE_DETAIL_DESCRIPTION;
}

const SITE_CHECK_RESULT_META: Record<string, { label: string; dot: string; textClass: string }> = {
  SUCCESS: {
    label: '访问正常',
    dot: 'var(--color-ok-dot)',
    textClass: 'text-[color:var(--color-ok)]',
  },
  FAILURE: {
    label: '访问失败',
    dot: 'var(--color-fail-dot)',
    textClass: 'text-[color:var(--color-fail)]',
  },
  TIMEOUT: {
    label: '请求超时',
    dot: 'var(--color-fail-dot)',
    textClass: 'text-[color:var(--color-fail)]',
  },
  DNS_ERROR: {
    label: 'DNS 异常',
    dot: 'var(--color-fail-dot)',
    textClass: 'text-[color:var(--color-fail)]',
  },
  SSL_ERROR: {
    label: '证书异常',
    dot: 'var(--color-warn-dot)',
    textClass: 'text-[color:var(--color-warn)]',
  },
  HTTP_ERROR: {
    label: 'HTTP 异常',
    dot: 'var(--color-warn-dot)',
    textClass: 'text-[color:var(--color-warn)]',
  },
  BLOCKED: {
    label: '访问受限',
    dot: 'var(--color-warn-dot)',
    textClass: 'text-[color:var(--color-warn)]',
  },
};

export function cloneSiteDetailPagedResult<TItem extends object>(
  source: PagedResult<TItem>,
): PagedResult<TItem> {
  return {
    items: source.items.map((item) => ({ ...item }) as TItem),
    pagination: { ...source.pagination },
  };
}

export function formatSiteDetailDateTime(value: string | null): string {
  if (!value) {
    return '未记录';
  }

  return new Date(value).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatSiteDetailStatusLabel(value: string): string {
  if (value === 'ERROR') {
    return '访问异常';
  }

  if (value === 'WARNING') {
    return '部分异常';
  }

  return '状态正常';
}

export function normalizeSiteCheckResult(result: string): string {
  if (result === 'OK') return 'SUCCESS';
  if (result === 'ERROR') return 'FAILURE';
  if (result === 'WARNING') return 'BLOCKED';
  return result || 'FAILURE';
}

export function formatSiteCheckResultLabel(result: string): string {
  const normalized = normalizeSiteCheckResult(result);
  return SITE_CHECK_RESULT_META[normalized]?.label ?? normalized;
}

export function formatSiteCheckRegionLabel(region: string): string {
  if (region === 'CN') return '国内';
  if (region === 'GLOBAL') return '国外';
  return region || '未知区域';
}

export function resolveSiteCheckTone(result: string): string {
  const normalized = normalizeSiteCheckResult(result);
  return SITE_CHECK_RESULT_META[normalized]?.dot ?? 'var(--color-warn-dot)';
}

export function resolveSiteCheckTextClass(result: string): string {
  const normalized = normalizeSiteCheckResult(result);
  return SITE_CHECK_RESULT_META[normalized]?.textClass ?? 'text-(--color-fg-2)';
}

export function resolveSiteStatusToneClass(value: string): string {
  if (value === 'OK') {
    return 'text-[color:var(--color-ok)]';
  }

  if (value === 'ERROR') {
    return 'text-[color:var(--color-fail)]';
  }

  return 'text-[color:var(--color-warn)]';
}

export function resolveHeartbeatSlotCount(viewportWidth: number): number {
  if (viewportWidth >= 1440) {
    return 30;
  }

  if (viewportWidth >= 1280) {
    return 24;
  }

  if (viewportWidth >= 1024) {
    return 20;
  }

  if (viewportWidth >= 768) {
    return 16;
  }

  if (viewportWidth >= 640) {
    return 12;
  }

  return 10;
}

export function buildHeartbeatChecks(checks: SiteCheckItem[], slotCount: number) {
  const recent = [...checks].slice(0, slotCount).reverse();
  const placeholders = Array.from(
    { length: Math.max(0, slotCount - recent.length) },
    (_, index) => ({
      id: `placeholder-${index}`,
      item: null as SiteCheckItem | null,
    }),
  );

  return [
    ...placeholders,
    ...recent.map((item) => ({
      id: item.id,
      item,
    })),
  ];
}

export function buildSiteCheckFacts(item: SiteCheckItem) {
  return [
    { label: '检测时间', value: formatSiteDetailDateTime(item.checkTime) },
    { label: '状态码', value: item.statusCode !== null ? String(item.statusCode) : '无' },
    { label: '响应耗时', value: item.responseTimeMs !== null ? `${item.responseTimeMs} ms` : '无' },
    { label: '检测耗时', value: item.durationMs !== null ? `${item.durationMs} ms` : '无' },
    { label: '最终地址', value: item.finalUrl ?? '无' },
    { label: '内容校验', value: item.contentVerified ? '通过' : '未通过' },
  ];
}

export interface SiteResourceLink {
  key: string;
  label: string;
  value: string;
  href: string;
  kind: 'rss' | 'sitemap' | 'external';
}

export function buildSiteResourceLinks(detail: SiteDetail) {
  const items: Array<{
    key: string;
    label: string;
    value: string;
    href: string;
    kind: 'rss' | 'sitemap' | 'external';
  }> = [];

  const feeds =
    detail.feeds.length > 0
      ? detail.feeds
      : detail.feedUrl
        ? [
            {
              name: null,
              url: detail.feedUrl,
              type: null,
              isDefault: true,
            },
          ]
        : [];

  const multipleFeeds = feeds.length > 1;

  for (const [index, feed] of feeds.entries()) {
    const feedName = feed.name?.trim() || `订阅${index + 1}`;

    items.push({
      key: `feed:${feed.url}`,
      label: multipleFeeds ? `RSS-${feedName}` : 'RSS',
      value: feed.url,
      href: feed.url,
      kind: 'rss',
    });
  }

  if (detail.sitemap) {
    items.push({
      key: 'sitemap',
      label: '站点地图',
      value: detail.sitemap,
      href: detail.sitemap,
      kind: 'sitemap',
    });
  }

  if (detail.linkPage) {
    items.push({
      key: 'links',
      label: '友链页',
      value: detail.linkPage,
      href: detail.linkPage,
      kind: 'external',
    });
  }

  return items;
}

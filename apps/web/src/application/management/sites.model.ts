import type { ManagedSite, SiteVisibility } from '../../api/managed-sites/managed-sites.types.ts';

export interface ManagedSitesQuery {
  readonly q: string;
  readonly visibility: SiteVisibility | '';
  readonly page: number;
}

export const siteVisibilityLabels: Readonly<Record<SiteVisibility, string>> = {
  VISIBLE: '正常收录',
  HIDDEN: '暂时隐藏',
  REMOVED: '已移除',
};

export const siteAccessScopeLabels: Readonly<Record<ManagedSite['access_scope'], string>> = {
  CN_ONLY: '中国大陆',
  GLOBAL_ONLY: '境外',
  ALL: '全球',
};

export function managedSitesQuery(parameters: URLSearchParams): ManagedSitesQuery {
  const q = [...(parameters.get('q') ?? '').trim()].slice(0, 160).join('');
  const requestedVisibility = parameters.get('visibility');
  const visibility =
    requestedVisibility === 'VISIBLE' ||
    requestedVisibility === 'HIDDEN' ||
    requestedVisibility === 'REMOVED'
      ? requestedVisibility
      : '';
  const requestedPage = Number(parameters.get('page') ?? '1');
  const page = Number.isSafeInteger(requestedPage) && requestedPage > 0 ? requestedPage : 1;
  return { q, visibility, page };
}

export function managedSitesPath(query: ManagedSitesQuery, page = query.page): string {
  const parameters = new URLSearchParams();
  if (query.q) parameters.set('q', query.q);
  if (query.visibility) parameters.set('visibility', query.visibility);
  if (page > 1) parameters.set('page', String(page));
  return `/management/sites${parameters.size ? `?${parameters}` : ''}`;
}

export function managedSiteDate(value: string): string {
  return new Date(value).toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    timeZone: 'UTC',
  });
}

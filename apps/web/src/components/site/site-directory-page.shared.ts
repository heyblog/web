import type {
  SiteDirectoryMeta,
  SiteDirectoryPreference,
  SiteDirectoryResult,
} from '@/application/site/site-directory.models';
import {
  parseSiteDirectoryStructuredSearch,
  type SiteDirectoryStructuredSearchState,
} from '@/application/site/site-directory.search';
import type { SiteDirectoryQueryState } from '@/application/site/site-directory.shared';

export const SITE_DIRECTORY_ACCESS_OPTIONS = [
  { key: '全球', label: '全球可访问' },
  { key: '大陆', label: '仅中国大陆可访问' },
  { key: '海外', label: '仅海外可访问' },
] as const;

export const SITE_DIRECTORY_BOOLEAN_OPTIONS = [
  { key: 'true', label: '是' },
  { key: 'false', label: '否' },
] as const;

export const SITE_DIRECTORY_SORT_OPTIONS = [
  { key: '', label: '默认排序' },
  { key: 'updated', label: '更新时间' },
  { key: 'joined', label: '加入时间' },
  { key: 'visits', label: '访问数' },
  { key: 'articles', label: '文章数' },
] as const;

export function cloneSiteDirectoryResult(source: SiteDirectoryResult): SiteDirectoryResult {
  return {
    items: source.items.map((item) => ({
      ...item,
      subTags: [...item.subTags],
      warningTags: item.warningTags.map((tag) => ({ ...tag })),
    })),
    pagination: { ...source.pagination },
    query: {
      ...source.query,
      main: [...source.query.main],
      sub: [...source.query.sub],
      warning: [...source.query.warning],
      program: [...source.query.program],
    },
  };
}

export function cloneSiteDirectoryMeta(source: SiteDirectoryMeta): SiteDirectoryMeta {
  return {
    stats: { ...source.stats },
    filters: {
      mainTags: source.filters.mainTags.map((item) => ({ ...item })),
      subTags: source.filters.subTags.map((item) => ({ ...item })),
      warningTags: source.filters.warningTags.map((item) => ({ ...item })),
      programs: source.filters.programs.map((item) => ({ ...item })),
    },
    defaults: { ...source.defaults },
  };
}

export function createInitialSiteDirectoryQuery(
  source: SiteDirectoryResult,
): SiteDirectoryQueryState {
  const nextResult = cloneSiteDirectoryResult(source);

  return {
    ...nextResult.query,
    page: nextResult.pagination.page,
    pageSize: nextResult.pagination.pageSize,
  };
}

export function cloneSiteDirectoryPreference(
  value: SiteDirectoryPreference | null | undefined,
): SiteDirectoryPreference | null {
  return value ? { ...value } : null;
}

export function cloneStructuredSearchState(
  value: SiteDirectoryStructuredSearchState,
): SiteDirectoryStructuredSearchState {
  return {
    keywords: [...value.keywords],
    main: [...value.main],
    sub: [...value.sub],
    warning: [...value.warning],
    program: [...value.program],
    site: [...value.site],
    domain: [...value.domain],
    access: [...value.access],
    rss: value.rss,
    featured: value.featured,
  };
}

export function createQueryFromSiteDirectorySearch(
  searchText: string,
  baseQuery: SiteDirectoryQueryState,
  overrides: Partial<SiteDirectoryQueryState> = {},
): SiteDirectoryQueryState {
  const parsed = parseSiteDirectoryStructuredSearch(searchText);

  return {
    ...baseQuery,
    q: searchText.trim(),
    main: parsed.main,
    sub: parsed.sub,
    warning: parsed.warning,
    program: parsed.program,
    ...overrides,
  };
}

export function toggleStructuredSelection(
  current: string[],
  value: string,
  multiple: boolean,
): string[] {
  if (multiple) {
    return current.includes(value) ? current.filter((item) => item !== value) : [...current, value];
  }

  return current.includes(value) ? [] : [value];
}

export function summarizeDirectorySelection(values: string[]): string {
  if (values.length === 0) {
    return '';
  }

  if (values.length === 1) {
    return values[0] ?? '';
  }

  return `${values[0]} +${values.length - 1}`;
}

export function formatDirectoryCount(value: number): string {
  return new Intl.NumberFormat('zh-CN').format(value);
}

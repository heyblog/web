import {
  compareNames,
  normalizeText,
  stableHash,
} from '@/application/public/usecase/public-site.directory.core';
import type {
  DirectoryState,
  PublicSiteDirectoryItem,
  PublicSiteTagState,
  SiteArchitectureFilterState,
} from '@/application/public/usecase/public-site.types';

export function collectSiteTags(
  rows: Array<{ site_id: string; tagName: string; tagType: string }>,
): Map<string, PublicSiteTagState> {
  const tagStateBySiteId = new Map<string, PublicSiteTagState>();

  for (const row of rows) {
    const current = tagStateBySiteId.get(row.site_id) ?? {
      primaryTag: null,
      subTags: [],
    };

    if (row.tagType === 'MAIN' && current.primaryTag === null) {
      current.primaryTag = row.tagName;
    }

    if (row.tagType === 'SUB' && !current.subTags.includes(row.tagName)) {
      current.subTags.push(row.tagName);
    }

    tagStateBySiteId.set(row.site_id, current);
  }

  for (const state of tagStateBySiteId.values()) {
    state.subTags.sort(compareNames);
  }

  return tagStateBySiteId;
}

function matchesKeywords(
  site: PublicSiteDirectoryItem,
  keywords: string[],
  extras: string[] = [],
): boolean {
  if (keywords.length === 0) {
    return true;
  }

  const haystack = [
    site.name,
    site.url,
    site.sign,
    site.primaryTag ?? '',
    ...site.subTags,
    ...site.warningTags.map((tag) => tag.name),
    ...extras,
  ]
    .join(' ')
    .toLowerCase();

  return keywords.every((keyword) => haystack.includes(keyword));
}

function extractSiteDomain(url: string): string {
  try {
    const target = new URL(url);
    return `${target.host}${target.pathname === '/' ? '' : target.pathname}`;
  } catch {
    return url;
  }
}

function includesAllTags(source: string[], target: string[]): boolean {
  if (target.length === 0) {
    return true;
  }

  const normalized = new Set(source.map(normalizeText));
  return target.every((item) => normalized.has(normalizeText(item)));
}

function includesAllText(source: string, target: string[]): boolean {
  if (target.length === 0) {
    return true;
  }

  const haystack = normalizeText(source);
  return target.every((value) => haystack.includes(normalizeText(value)));
}

function getAccessScopeSearchTerms(accessScope: string): string[] {
  if (accessScope === 'CN_ONLY') {
    return ['cn_only', 'cn', 'china', '中国', '中国大陆', '大陆', '国内', '仅中国大陆可访问'];
  }

  if (accessScope === 'NON_CN_ONLY') {
    return ['non_cn_only', 'non_cn', 'global', '海外', '国际', '仅海外可访问'];
  }

  return ['all', 'both', 'global', 'worldwide', '全球', '全球可访问'];
}

function matchesAccessScope(accessScope: string, target: string[]): boolean {
  if (target.length === 0) {
    return true;
  }

  const haystack = getAccessScopeSearchTerms(accessScope).join(' ').toLowerCase();
  return target.every((value) => haystack.includes(normalizeText(value)));
}

function matchesPrimaryTag(site: PublicSiteDirectoryItem, target: string[]): boolean {
  return (
    target.length === 0 ||
    target.some((value) => normalizeText(site.primaryTag ?? '') === normalizeText(value))
  );
}

function matchesProgram(
  siteId: string,
  target: string[],
  architecture: SiteArchitectureFilterState,
): boolean {
  if (target.length === 0) {
    return true;
  }

  const programName = architecture.programBySiteId.get(siteId) ?? '';
  return target.some((value) => normalizeText(programName) === normalizeText(value));
}

function matchesStatusMode(
  site: PublicSiteDirectoryItem,
  statusMode: DirectoryState['statusMode'],
) {
  const isNormal = site.status === 'OK';

  if (statusMode === 'normal') {
    return isNormal;
  }

  return !isNormal;
}

function matchesStructuredFields(
  site: PublicSiteDirectoryItem,
  query: DirectoryState,
  architecture: SiteArchitectureFilterState,
): boolean {
  return (
    matchesPrimaryTag(site, query.main) &&
    includesAllTags(site.subTags, query.sub) &&
    includesAllTags(
      site.warningTags.map((tag) => tag.name),
      query.warning,
    ) &&
    matchesProgram(site.id, query.program, architecture) &&
    includesAllText(site.name, query.site) &&
    includesAllText(`${extractSiteDomain(site.url)} ${site.url}`, query.domain) &&
    matchesAccessScope(site.accessScope, query.access)
  );
}

export function matchesDirectoryFilters(
  site: PublicSiteDirectoryItem,
  query: DirectoryState,
  architecture: SiteArchitectureFilterState,
): boolean {
  const programName = architecture.programBySiteId.get(site.id) ?? '';

  if (!matchesStatusMode(site, query.statusMode)) {
    return false;
  }

  if (!matchesKeywords(site, query.keywords, [programName])) {
    return false;
  }

  if (!matchesStructuredFields(site, query, architecture)) {
    return false;
  }

  if (query.rss !== null && Boolean(site.feedUrl) !== query.rss) {
    return false;
  }

  return query.featured === null || site.featured === query.featured;
}

function readTimeValue(value: string | null | undefined): number {
  if (!value) {
    return Number.NEGATIVE_INFINITY;
  }

  const parsed = new Date(value).getTime();
  return Number.isNaN(parsed) ? Number.NEGATIVE_INFINITY : parsed;
}

function compareBySort(
  left: PublicSiteDirectoryItem,
  right: PublicSiteDirectoryItem,
  query: DirectoryState,
): number {
  if (query.sort === 'updated') {
    const latestDiff =
      readTimeValue(left.latestPublishedTime) - readTimeValue(right.latestPublishedTime);

    return latestDiff !== 0
      ? latestDiff
      : readTimeValue(left.updateTime) - readTimeValue(right.updateTime);
  }

  if (query.sort === 'joined') {
    return readTimeValue(left.joinTime) - readTimeValue(right.joinTime);
  }

  if (query.sort === 'visits') {
    return left.visitCount - right.visitCount;
  }

  if (query.sort === 'articles') {
    return left.articleCount - right.articleCount;
  }

  return 0;
}

export function sortDirectoryItems(
  items: PublicSiteDirectoryItem[],
  query: DirectoryState,
): PublicSiteDirectoryItem[] {
  const draft = [...items];

  if (query.sort) {
    const factor = query.order === 'asc' ? 1 : -1;

    draft.sort((left, right) => {
      const compare = compareBySort(left, right, query);
      return compare !== 0 ? compare * factor : compareNames(left.name, right.name);
    });

    return draft;
  }

  if (query.random) {
    draft.sort((left, right) => {
      const leftHash = stableHash(`${query.randomSeed}:${left.id}`);
      const rightHash = stableHash(`${query.randomSeed}:${right.id}`);

      if (leftHash !== rightHash) {
        return leftHash - rightHash;
      }

      return compareNames(left.name, right.name);
    });

    return draft;
  }

  return draft;
}

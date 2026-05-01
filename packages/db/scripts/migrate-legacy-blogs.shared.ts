import { type FromSourceKey, MAIN_TAGS } from '../src/constants/site';

import { type Blogs } from './blogs';

export type OldBlog = typeof Blogs.$inferSelect;
export type Transaction = any;

export type ClassificationStatus = 'COMPLETE' | 'NEEDS_REVIEW';

export type MainTagResolution = {
  label: string;
  classificationStatus: ClassificationStatus;
};

export type LegacyFeed = {
  name: string;
  url: string;
  isDefault: boolean;
};

export type MigrationStats = {
  processed: number;
  created: number;
  updated: number;
  skippedInvalidUrl: number;
  skippedDuplicateUrl: number;
  withProgram: number;
  withoutProgram: number;
  unknownMainTag: number;
  unknownSource: number;
};

export const DEFAULT_BATCH_SIZE = 200;

export const HELP = `
Legacy blogs -> new schema migration

Usage:
  OLD_DATABASE_URL=postgresql://... \\
  NEW_DATABASE_URL=postgresql://... \\
  pnpm tsx ./scripts/migrate-legacy-blogs.ts

Do not write:
  OLD_DATABASE_URL=DATABASE_URL=postgresql://...
  NEW_DATABASE_URL=DATABASE_URL=postgresql://...

Environment variables:
  OLD_DATABASE_URL
    Required. Connection string for the old database that still has the blogs table.

  NEW_DATABASE_URL / DATABASE_URL
    Required. Connection string for the new database that uses the current schema.

  MIGRATE_LEGACY_DRY_RUN
    Optional. Set to 1 / true to only read and summarize old data without writing.

  MIGRATE_LEGACY_BATCH_SIZE
    Optional. Batch size for write transactions. Defaults to 200.
`.trim();

const OLD_MAIN_TAG_LABEL_TO_NEW_LABEL = new Map<string, string>([
  ['生活', MAIN_TAGS.LIFE.label],
  ['技术', MAIN_TAGS.TECH.label],
  ['知识', MAIN_TAGS.KNOWLEDGE.label],
  ['整合', MAIN_TAGS.CURATION.label],
  ['采集', MAIN_TAGS.COLLECTION.label],
  ['综合', MAIN_TAGS.GENERAL.label],
]);

const OLD_FROM_SOURCE_TO_NEW_KEY: Record<string, FromSourceKey> = {
  CIB: 'CIB',
  BoYouQuan: 'BO_YOU_QUAN',
  BlogFinder: 'BLOG_FINDER',
  BKZ: 'BKZ',
  Travellings: 'TRAVELLINGS',
  WebSubmit: 'WEB_SUBMIT',
  AdminAdd: 'WEB_SUBMIT',
  LinkPageSearch: 'LINK_PAGE_SEARCH',
  OldData: 'OLD_DATA',
};

export const isTruthy = (value: string | undefined): boolean =>
  ['1', 'true', 'yes', 'on'].includes((value ?? '').trim().toLowerCase());

export const normalizeLabel = (value: string | null | undefined): string | null => {
  if (typeof value !== 'string') {
    return null;
  }

  const normalized = value.trim();
  return normalized.length > 0 ? normalized : null;
};

export const normalizeProgramToken = (value: string | null | undefined): string | null => {
  const normalized = normalizeLabel(value);
  if (!normalized) {
    return null;
  }

  const compact = normalized.toLocaleLowerCase('zh-CN').replace(/[^\p{L}\p{N}]+/gu, '');
  return compact || normalized.toLocaleLowerCase('zh-CN');
};

export const mapMainTag = (oldLabel: string | null): MainTagResolution => {
  const normalized = normalizeLabel(oldLabel);
  const mapped = normalized ? OLD_MAIN_TAG_LABEL_TO_NEW_LABEL.get(normalized) : null;

  if (mapped) {
    return { label: mapped, classificationStatus: 'COMPLETE' };
  }

  return {
    label: MAIN_TAGS.GENERAL.label,
    classificationStatus: 'NEEDS_REVIEW',
  };
};

export const mapFromSources = (
  oldSources: string[] | null,
): { values: FromSourceKey[]; unknownCount: number } => {
  const mapped = new Set<FromSourceKey>();
  let unknownCount = 0;

  for (const oldSource of oldSources ?? []) {
    const normalized = OLD_FROM_SOURCE_TO_NEW_KEY[oldSource];
    if (normalized) {
      mapped.add(normalized);
      continue;
    }
    unknownCount += 1;
  }

  return { values: Array.from(mapped), unknownCount };
};

export const mapStatus = (value: string | null): 'OK' | 'WARNING' | 'ERROR' => {
  if (value === 'SSLERROR') {
    return 'WARNING';
  }

  if (value === 'ERROR') {
    return 'ERROR';
  }

  return 'OK';
};

export const sanitizeFeeds = (feeds: OldBlog['feed']): LegacyFeed[] => {
  const dedup = new Set<string>();
  const normalized: LegacyFeed[] = [];

  for (const feed of feeds ?? []) {
    const url = normalizeLabel(feed?.url);
    if (!url || dedup.has(url)) {
      continue;
    }

    dedup.add(url);
    normalized.push({
      name: normalizeLabel(feed?.name) ?? '',
      url,
      isDefault: false,
    });
  }

  return normalized.map((feed, index) => ({
    ...feed,
    isDefault: normalized.length === 1 || index === 0,
  }));
};

const normalizeConnectionString = (value: string | undefined, envName: string): string => {
  const raw = value?.trim();
  if (!raw) {
    throw new Error(`Missing ${envName}`);
  }

  const unwrapped = raw.replace(/^['"]|['"]$/g, '');
  const normalized = unwrapped.replace(/^[A-Z_][A-Z0-9_]*=(postgres(?:ql)?:\/\/.*)$/i, '$1');
  const lower = normalized.toLowerCase();

  if (!lower.startsWith('postgres://') && !lower.startsWith('postgresql://')) {
    throw new Error(`${envName} must be a PostgreSQL URL.`);
  }

  return normalized;
};

export const getOldDatabaseUrl = (): string =>
  normalizeConnectionString(process.env.OLD_DATABASE_URL?.trim(), 'OLD_DATABASE_URL');

export const getNewDatabaseUrl = (): string =>
  normalizeConnectionString(
    process.env.NEW_DATABASE_URL?.trim() || process.env.DATABASE_URL?.trim(),
    'NEW_DATABASE_URL or DATABASE_URL',
  );

export const readBatchSize = (): number => {
  const raw = process.env.MIGRATE_LEGACY_BATCH_SIZE?.trim();
  if (!raw) {
    return DEFAULT_BATCH_SIZE;
  }

  const parsed = Number(raw);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    throw new Error('MIGRATE_LEGACY_BATCH_SIZE must be a positive number');
  }

  return Math.floor(parsed);
};

export const chunkBy = <T>(items: T[], size: number): T[][] => {
  const chunks: T[][] = [];
  for (let index = 0; index < items.length; index += size) {
    chunks.push(items.slice(index, index + size));
  }
  return chunks;
};

export const dedupeBlogsByUrl = (
  blogs: OldBlog[],
): { deduped: OldBlog[]; duplicateRows: number } => {
  const byUrl = new Map<string, OldBlog>();
  let duplicateRows = 0;

  for (const blog of blogs) {
    const blogUrl = normalizeLabel(blog.url);
    if (!blogUrl) {
      continue;
    }

    if (byUrl.has(blogUrl)) {
      duplicateRows += 1;
    }
    byUrl.set(blogUrl, blog);
  }

  return { deduped: Array.from(byUrl.values()), duplicateRows };
};

export const collectSubTags = (blogs: OldBlog[]): string[] => {
  const values = new Set<string>();

  for (const blog of blogs) {
    for (const rawTag of blog.sub_tag ?? []) {
      const normalized = normalizeLabel(rawTag);
      if (normalized) {
        values.add(normalized);
      }
    }
  }

  return Array.from(values);
};

export const collectProgramNames = (blogs: OldBlog[]): string[] => {
  const values = new Set<string>();

  for (const blog of blogs) {
    const normalized = normalizeLabel(blog.arch);
    if (normalized) {
      values.add(normalized);
    }
  }

  return Array.from(values);
};

export const buildMainTagDescriptions = (): Map<string, string> =>
  new Map(Object.values(MAIN_TAGS).map((tag) => [tag.label, tag.description] as const));

export const createStats = (): MigrationStats => ({
  processed: 0,
  created: 0,
  updated: 0,
  skippedInvalidUrl: 0,
  skippedDuplicateUrl: 0,
  withProgram: 0,
  withoutProgram: 0,
  unknownMainTag: 0,
  unknownSource: 0,
});

export const printPreflightSummary = (
  allBlogs: OldBlog[],
  dedupedBlogs: OldBlog[],
  duplicateRows: number,
  batchSize: number,
): void => {
  const fromSources = new Set<string>();
  const invalidUrlRows = allBlogs.filter((blog) => !normalizeLabel(blog.url)).length;
  const needsReviewMainTags = dedupedBlogs.filter(
    (blog) => mapMainTag(blog.main_tag).classificationStatus === 'NEEDS_REVIEW',
  ).length;

  for (const blog of dedupedBlogs) {
    for (const source of blog.from ?? []) {
      fromSources.add(source);
    }
  }

  console.log(
    JSON.stringify(
      {
        totalRows: allBlogs.length,
        uniqueValidUrlRows: dedupedBlogs.length,
        duplicateUrlRows: duplicateRows,
        batchSize,
        plannedBatches: Math.ceil(dedupedBlogs.length / batchSize),
        invalidUrlRows,
        uniqueSubTags: collectSubTags(dedupedBlogs).length,
        uniquePrograms: collectProgramNames(dedupedBlogs).length,
        fromSources: Array.from(fromSources).sort(),
        needsReviewMainTags,
      },
      null,
      2,
    ),
  );
};

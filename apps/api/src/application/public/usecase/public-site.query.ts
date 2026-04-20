import { PublicSiteDirectoryReadView, SiteProgramSummaryView, TagDefinitions } from '@zhblogs/db';

import { asc, eq, type SQL, sql } from 'drizzle-orm';
import type { FastifyInstance } from 'fastify';

import {
  createSiteSlug,
  normalizeText,
} from '@/application/public/usecase/public-site.directory.core';
import type {
  DirectoryState,
  PublicSiteDetail,
  PublicSiteDirectoryItem,
  PublicSiteDirectoryMeta,
  PublicSiteWarningTag,
} from '@/application/public/usecase/public-site.types';

type DirectoryQueryRow = {
  siteId: string;
  bid: string | null;
  name: string;
  url: string;
  sign: string | null;
  feeds: Array<{
    name?: string | null;
    url?: string | null;
    type?: string | null;
    isDefault?: boolean | null;
  }> | null;
  feedUrl: string | null;
  sitemap: string | null;
  linkPage: string | null;
  featured: boolean;
  status: string;
  accessScope: string;
  joinTime: Date | string;
  updateTime: Date | string;
  reason: string | null;
  articleCount: number | string | null;
  latestPublishedTime: Date | string | null;
  visitCount: number | string | null;
  primaryTag: string | null;
  subTags: string[] | string | null;
  warningTags: PublicSiteWarningTag[] | string | null;
  warningNames: string[] | string | null;
  programId: string | null;
  programName: string | null;
  programIsOpenSource: boolean | null;
  websiteUrl: string | null;
  repoUrl: string | null;
};

type CountRow = {
  totalItems: number;
};

type DirectoryStatsRow = {
  totalSites: number;
  normalSites: number;
  abnormalSites: number;
  rssSites: number;
};

type ProgramFilterRow = {
  id: string;
  name: string;
};

const DIRECTORY_BASE_SELECT = sql<DirectoryQueryRow>`
  select
    site_id as "siteId",
    bid as "bid",
    name as "name",
    url as "url",
    sign as "sign",
    feeds as "feeds",
    feed_url as "feedUrl",
    sitemap as "sitemap",
    link_page as "linkPage",
    featured as "featured",
    status as "status",
    access_scope as "accessScope",
    join_time as "joinTime",
    update_time as "updateTime",
    reason as "reason",
    article_count as "articleCount",
    latest_published_time as "latestPublishedTime",
    visit_count as "visitCount",
    primary_tag as "primaryTag",
    sub_tags as "subTags",
    warning_tags as "warningTags",
    warning_names as "warningNames",
    program_id as "programId",
    program_name as "programName",
    program_is_open_source as "programIsOpenSource",
    website_url as "websiteUrl",
    repo_url as "repoUrl"
  from ${PublicSiteDirectoryReadView}
`;

const DIRECTORY_KEYWORD_HAYSTACK = sql`
  lower(
    concat_ws(
      ' ',
      name,
      url,
      coalesce(sign, ''),
      coalesce(primary_tag, ''),
      array_to_string(sub_tags, ' '),
      array_to_string(warning_names, ' '),
      coalesce(program_name, '')
    )
  )
`;

const DIRECTORY_ACCESS_HAYSTACK = sql`
  lower(
    case
      when access_scope = 'CN_ONLY'
      then 'cn_only cn china 中国 中国大陆 大陆 国内 仅中国大陆可访问'
      when access_scope = 'NON_CN_ONLY'
      then 'non_cn_only non_cn global 海外 国际 仅海外可访问'
      else 'all both global worldwide 全球 全球可访问'
    end
  )
`;

function readQueryRows<T extends object>(value: unknown): T[] {
  if (Array.isArray(value)) {
    return value as T[];
  }

  if (value && typeof value === 'object' && 'rows' in value && Array.isArray(value.rows)) {
    return value.rows as T[];
  }

  return [];
}

function readNumber(value: number | string | null | undefined): number {
  if (typeof value === 'number') {
    return value;
  }

  if (typeof value === 'string') {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }

  return 0;
}

function readIsoString(value: Date | string | null | undefined): string | null {
  if (!value) {
    return null;
  }

  const target = value instanceof Date ? value : new Date(value);
  return Number.isNaN(target.getTime()) ? null : target.toISOString();
}

function readTextArray(value: string[] | string | null | undefined): string[] {
  if (Array.isArray(value)) {
    return value.filter((item): item is string => typeof item === 'string');
  }

  if (typeof value === 'string') {
    try {
      const parsed = JSON.parse(value) as unknown;
      return Array.isArray(parsed)
        ? parsed.filter((item): item is string => typeof item === 'string')
        : [];
    } catch {
      return [];
    }
  }

  return [];
}

function readWarningTags(
  value: PublicSiteWarningTag[] | string | null | undefined,
): PublicSiteWarningTag[] {
  if (Array.isArray(value)) {
    return value.map((item) => ({
      machineKey: item.machineKey,
      name: item.name,
      description: item.description ?? null,
    }));
  }

  if (typeof value === 'string') {
    try {
      const parsed = JSON.parse(value) as unknown;
      return Array.isArray(parsed)
        ? parsed.flatMap((item) =>
            item &&
            typeof item === 'object' &&
            'machineKey' in item &&
            typeof item.machineKey === 'string' &&
            'name' in item &&
            typeof item.name === 'string'
              ? [
                  {
                    machineKey: item.machineKey as PublicSiteWarningTag['machineKey'],
                    name: item.name,
                    description:
                      'description' in item && typeof item.description === 'string'
                        ? item.description
                        : 'description' in item
                          ? (item.description ?? null)
                          : null,
                  },
                ]
              : [],
          )
        : [];
    } catch {
      return [];
    }
  }

  return [];
}

function buildInClause(values: string[]): SQL {
  return sql.join(
    values.map((value) => sql`${normalizeText(value)}`),
    sql`, `,
  );
}

function buildArrayContainsClause(columnName: 'sub_tags' | 'warning_names', value: string): SQL {
  const normalized = normalizeText(value);
  const column = sql.raw(columnName);

  return sql`
    exists (
      select 1
      from unnest(${column}) as item(value)
      where lower(item.value) = ${normalized}
    )
  `;
}

function buildStructuredTextClause(target: SQL, value: string): SQL {
  return sql`${target} like ${`%${normalizeText(value)}%`}`;
}

function buildDirectoryFilters(query: DirectoryState): SQL {
  const conditions: SQL[] = [
    query.statusMode === 'normal' ? sql`status = 'OK'` : sql`status <> 'OK'`,
  ];

  for (const keyword of query.keywords) {
    conditions.push(buildStructuredTextClause(DIRECTORY_KEYWORD_HAYSTACK, keyword));
  }

  if (query.main.length > 0) {
    conditions.push(sql`lower(coalesce(primary_tag, '')) in (${buildInClause(query.main)})`);
  }

  for (const value of query.sub) {
    conditions.push(buildArrayContainsClause('sub_tags', value));
  }

  for (const value of query.warning) {
    conditions.push(buildArrayContainsClause('warning_names', value));
  }

  if (query.program.length > 0) {
    conditions.push(sql`lower(coalesce(program_name, '')) in (${buildInClause(query.program)})`);
  }

  for (const value of query.site) {
    conditions.push(buildStructuredTextClause(sql`lower(name)`, value));
  }

  for (const value of query.domain) {
    conditions.push(buildStructuredTextClause(sql`lower(url)`, value));
  }

  for (const value of query.access) {
    conditions.push(buildStructuredTextClause(DIRECTORY_ACCESS_HAYSTACK, value));
  }

  if (query.rss !== null) {
    conditions.push(query.rss ? sql`feed_url is not null` : sql`feed_url is null`);
  }

  if (query.featured !== null) {
    conditions.push(sql`featured = ${query.featured}`);
  }

  return sql`where ${sql.join(conditions, sql` and `)}`;
}

function buildDirectoryOrderBy(query: DirectoryState): SQL {
  if (query.sort === 'updated') {
    return sql`
      order by
        coalesce(latest_published_time, update_time) ${sql.raw(query.order)},
        update_time ${sql.raw(query.order)},
        lower(name) asc
    `;
  }

  if (query.sort === 'joined') {
    return sql`
      order by
        join_time ${sql.raw(query.order)},
        lower(name) asc
    `;
  }

  if (query.sort === 'visits') {
    return sql`
      order by
        visit_count ${sql.raw(query.order)},
        lower(name) asc
    `;
  }

  if (query.sort === 'articles') {
    return sql`
      order by
        article_count ${sql.raw(query.order)},
        lower(name) asc
    `;
  }

  if (query.random) {
    return sql`
      order by
        md5(${query.randomSeed} || ':' || site_id::text) asc,
        lower(name) asc
    `;
  }

  return sql`
    order by
      featured desc,
      update_time desc,
      join_time desc,
      lower(name) asc
  `;
}

function mapDirectoryRow(row: DirectoryQueryRow): PublicSiteDirectoryItem {
  return {
    id: row.siteId,
    bid: row.bid,
    slug: createSiteSlug({ id: row.siteId, bid: row.bid, name: row.name }),
    name: row.name,
    url: row.url,
    sign: row.sign ?? '',
    feedUrl: row.feedUrl ?? null,
    sitemap: row.sitemap ?? null,
    linkPage: row.linkPage ?? null,
    featured: row.featured === true,
    status: row.status,
    accessScope: row.accessScope,
    joinTime: readIsoString(row.joinTime) ?? new Date(0).toISOString(),
    updateTime: readIsoString(row.updateTime) ?? new Date(0).toISOString(),
    latestPublishedTime: readIsoString(row.latestPublishedTime),
    articleCount: readNumber(row.articleCount),
    visitCount: readNumber(row.visitCount),
    primaryTag: row.primaryTag ?? null,
    subTags: readTextArray(row.subTags),
    warningTags: readWarningTags(row.warningTags),
  };
}

function mapDirectoryDetail(
  row: DirectoryQueryRow,
  heartbeatChecks: PublicSiteDetail['heartbeatChecks'],
): PublicSiteDetail {
  const base = mapDirectoryRow(row);

  return {
    ...base,
    reason: row.reason ?? null,
    feeds: (Array.isArray(row.feeds) ? row.feeds : []).flatMap((feed) =>
      feed?.url
        ? [
            {
              name: feed.name?.trim() || null,
              url: feed.url,
              type: feed.type ?? null,
              isDefault: feed.isDefault === true,
            },
          ]
        : [],
    ),
    architecture: {
      program: row.programId
        ? {
            id: row.programId,
            name: row.programName ?? '',
            isOpenSource: row.programIsOpenSource === true,
            websiteUrl: row.websiteUrl ?? null,
            repoUrl: row.repoUrl ?? null,
          }
        : null,
    },
    heartbeatChecks,
  };
}

export async function loadDirectoryStats(
  app: FastifyInstance,
): Promise<PublicSiteDirectoryMeta['stats']> {
  const query = await app.db.read.execute(sql<DirectoryStatsRow>`
    select
      count(*)::int as "totalSites",
      count(*) filter (where status = 'OK')::int as "normalSites",
      count(*) filter (where status <> 'OK')::int as "abnormalSites",
      count(*) filter (where feed_url is not null)::int as "rssSites"
    from ${PublicSiteDirectoryReadView}
  `);
  const row = readQueryRows<DirectoryStatsRow>(query)[0];

  return {
    totalSites: row?.totalSites ?? 0,
    normalSites: row?.normalSites ?? 0,
    abnormalSites: row?.abnormalSites ?? 0,
    rssSites: row?.rssSites ?? 0,
  };
}

export async function loadProgramFilters(
  app: FastifyInstance,
): Promise<PublicSiteDirectoryMeta['filters']['programs']> {
  const query = await app.db.read.execute(sql<ProgramFilterRow>`
    select distinct
      program_id as "id",
      program_name as "name"
    from ${SiteProgramSummaryView}
    order by program_name asc
  `);

  return readQueryRows<ProgramFilterRow>(query).flatMap((row) =>
    row.id && row.name
      ? [
          {
            id: row.id,
            name: row.name,
          },
        ]
      : [],
  );
}

export async function loadTagFilters(
  app: FastifyInstance,
): Promise<Omit<PublicSiteDirectoryMeta['filters'], 'programs'>> {
  const rows = await app.db.read
    .select({
      id: TagDefinitions.id,
      name: TagDefinitions.name,
      machineKey: TagDefinitions.machine_key,
      tagType: TagDefinitions.tag_type,
    })
    .from(TagDefinitions)
    .where(eq(TagDefinitions.is_enabled, true))
    .orderBy(asc(TagDefinitions.tag_type), asc(TagDefinitions.name));

  return {
    mainTags: rows
      .filter((row) => row.tagType === 'MAIN')
      .map((row) => ({ id: row.id, name: row.name })),
    subTags: rows
      .filter((row) => row.tagType === 'SUB')
      .map((row) => ({ id: row.id, name: row.name })),
    warningTags: rows
      .filter((row) => row.tagType === 'WARNING')
      .map((row) => ({ id: row.id, machineKey: row.machineKey ?? null, name: row.name })),
  };
}

export async function countFilteredDirectoryItems(
  app: FastifyInstance,
  query: DirectoryState,
): Promise<number> {
  const filters = buildDirectoryFilters(query);
  const result = await app.db.read.execute(sql<CountRow>`
    select count(*)::int as "totalItems"
    from ${PublicSiteDirectoryReadView}
    ${filters}
  `);

  return readQueryRows<CountRow>(result)[0]?.totalItems ?? 0;
}

export async function loadFilteredDirectoryItems(
  app: FastifyInstance,
  query: DirectoryState,
  page: number,
): Promise<PublicSiteDirectoryItem[]> {
  const filters = buildDirectoryFilters(query);
  const orderBy = buildDirectoryOrderBy(query);
  const offset = Math.max(0, (page - 1) * query.pageSize);
  const result = await app.db.read.execute(sql<DirectoryQueryRow>`
    ${DIRECTORY_BASE_SELECT}
    ${filters}
    ${orderBy}
    limit ${query.pageSize}
    offset ${offset}
  `);

  return readQueryRows<DirectoryQueryRow>(result).map(mapDirectoryRow);
}

export async function loadDirectoryItemBySlug(
  app: FastifyInstance,
  slug: string,
): Promise<PublicSiteDirectoryItem | null> {
  const result = await app.db.read.execute(sql<DirectoryQueryRow>`
    ${DIRECTORY_BASE_SELECT}
    where site_id::text = ${slug}
    limit 1
  `);

  const row = readQueryRows<DirectoryQueryRow>(result)[0];
  return row ? mapDirectoryRow(row) : null;
}

export async function loadDirectoryDetailBySlug(
  app: FastifyInstance,
  slug: string,
  heartbeatChecks: PublicSiteDetail['heartbeatChecks'],
): Promise<PublicSiteDetail | null> {
  const result = await app.db.read.execute(sql<DirectoryQueryRow>`
    ${DIRECTORY_BASE_SELECT}
    where site_id::text = ${slug}
    limit 1
  `);

  const row = readQueryRows<DirectoryQueryRow>(result)[0];
  return row ? mapDirectoryDetail(row, heartbeatChecks) : null;
}

export async function loadRandomDirectoryItem(
  app: FastifyInstance,
  options: {
    seed: string;
    recommend: boolean;
    type: string;
  },
): Promise<PublicSiteDirectoryItem | null> {
  const conditions: SQL[] = [
    sql`status = 'OK'`,
    sql`coalesce(jsonb_array_length(warning_tags), 0) = 0`,
  ];

  if (options.recommend) {
    conditions.push(sql`featured = true`);
  }

  if (options.type.trim()) {
    conditions.push(sql`lower(coalesce(primary_tag, '')) = ${normalizeText(options.type)}`);
  }

  const whereClause = sql`where ${sql.join(conditions, sql` and `)}`;
  const result = await app.db.read.execute(sql<DirectoryQueryRow>`
    ${DIRECTORY_BASE_SELECT}
    ${whereClause}
    order by md5(${options.seed} || ':' || site_id::text) asc, lower(name) asc
    limit 1
  `);

  const row = readQueryRows<DirectoryQueryRow>(result)[0];
  return row ? mapDirectoryRow(row) : null;
}

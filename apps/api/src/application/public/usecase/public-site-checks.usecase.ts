import { SiteCheckRuns } from '@zhblogs/db';

import { sql } from 'drizzle-orm';
import type { FastifyInstance } from 'fastify';

import {
  loadVersionedPublicCache,
  PUBLIC_CACHE_TTL,
} from '@/application/public/usecase/public-cache.usecase';
import type {
  PublicSiteCheckItem,
  PublicSiteDetailTabPage,
} from '@/application/public/usecase/public-site.types';
import { resolvePublicSiteBySlug } from '@/application/public/usecase/public-site.usecase';

type SiteCheckQueryRow = {
  id: string;
  region: string;
  result: string;
  statusCode: number | null;
  responseTimeMs: number | null;
  durationMs: number | null;
  message: string | null;
  finalUrl: string | null;
  contentVerified: boolean;
  checkTime: Date;
};

type CountRow = {
  totalItems: number;
};

export async function loadPublicSiteChecks(
  app: FastifyInstance,
  slug: string,
  page = 1,
  pageSize = 20,
): Promise<PublicSiteDetailTabPage<PublicSiteCheckItem> | null> {
  const { normalizedPage, normalizedPageSize } = normalizePaging(page, pageSize);

  return loadVersionedPublicCache(app, {
    namespace: 'site',
    suffix: `checks:${slug}:${normalizedPage}:${normalizedPageSize}`,
    ttlSeconds: PUBLIC_CACHE_TTL.checks,
    loader: async () => {
      const totalItems = await countSiteChecks(app, slug);

      if (totalItems === 0) {
        const site = await resolvePublicSiteBySlug(app, slug);

        if (!site) {
          return null;
        }
      }

      const totalPages = Math.max(1, Math.ceil(totalItems / normalizedPageSize));
      const currentPage = Math.min(normalizedPage, totalPages);
      const offset = (currentPage - 1) * normalizedPageSize;
      const rows = await querySiteChecks(app, slug, normalizedPageSize, offset, totalItems);

      return createPagedResult(
        rows.map(mapPublicSiteCheck),
        currentPage,
        normalizedPageSize,
        totalItems,
      );
    },
  });
}

export async function loadRecentPublicSiteChecks(
  app: FastifyInstance,
  siteId: string,
  limit = 30,
): Promise<PublicSiteCheckItem[]> {
  const rows = await querySiteChecks(app, siteId, Math.max(1, limit), 0);
  return rows.map(mapPublicSiteCheck);
}

function normalizePaging(page: number, pageSize: number) {
  const normalizedPage = Math.max(1, page);
  const normalizedPageSize = Math.min(50, Math.max(10, pageSize));
  return { normalizedPage, normalizedPageSize };
}

function createPagedResult<T>(
  items: T[],
  page: number,
  pageSize: number,
  totalItems: number,
): PublicSiteDetailTabPage<T> {
  return {
    items,
    pagination: {
      page,
      pageSize,
      totalItems,
      totalPages: Math.max(1, Math.ceil(totalItems / pageSize)),
    },
  };
}

async function countExpandedSiteChecks(app: FastifyInstance, siteId: string): Promise<number> {
  const query = await app.db.read.execute(sql<CountRow>`
    select coalesce(
      sum(
        case
          when jsonb_typeof(${SiteCheckRuns.probe_summary}) = 'array'
            and jsonb_array_length(${SiteCheckRuns.probe_summary}) > 0
          then jsonb_array_length(${SiteCheckRuns.probe_summary})
          else 1
        end
      ),
      0
    )::int as "totalItems"
    from ${SiteCheckRuns}
    where ${SiteCheckRuns.site_id} = ${siteId}
  `);

  return readQueryRows<CountRow>(query)[0]?.totalItems ?? 0;
}

async function countSiteCheckRuns(app: FastifyInstance, siteId: string): Promise<number> {
  const query = await app.db.read.execute(sql<CountRow>`
    select count(*)::int as "totalItems"
    from ${SiteCheckRuns}
    where ${SiteCheckRuns.site_id} = ${siteId}
  `);

  return readQueryRows<CountRow>(query)[0]?.totalItems ?? 0;
}

async function countSiteChecks(app: FastifyInstance, siteId: string): Promise<number> {
  try {
    const totalItems = await countExpandedSiteChecks(app, siteId);

    if (totalItems > 0) {
      return totalItems;
    }
  } catch (error) {
    app.log.warn(
      { siteId, error },
      'failed to count expanded public site checks, fallback to runs',
    );
  }

  return countSiteCheckRuns(app, siteId);
}

async function querySiteChecks(
  app: FastifyInstance,
  siteId: string,
  limit: number,
  offset: number,
  knownTotalItems?: number,
): Promise<SiteCheckQueryRow[]> {
  const expandedRows = await tryQueryExpandedSiteChecks(
    app,
    siteId,
    limit,
    offset,
    knownTotalItems,
  );

  if (expandedRows) {
    return expandedRows;
  }

  return querySummarySiteChecks(app, siteId, limit, offset);
}

async function tryQueryExpandedSiteChecks(
  app: FastifyInstance,
  siteId: string,
  limit: number,
  offset: number,
  knownTotalItems?: number,
): Promise<SiteCheckQueryRow[] | null> {
  try {
    const totalItems = knownTotalItems ?? (await countExpandedSiteChecks(app, siteId));
    const rows = await queryExpandedSiteChecks(app, siteId, limit, offset);

    if (rows.length > 0 || totalItems === 0) {
      return rows;
    }
  } catch (error) {
    app.log.warn(
      { siteId, error },
      'failed to expand public site checks, fallback to summary rows',
    );
  }

  return null;
}

async function queryExpandedSiteChecks(
  app: FastifyInstance,
  siteId: string,
  limit: number,
  offset: number,
): Promise<SiteCheckQueryRow[]> {
  const query = await app.db.read.execute(sql<SiteCheckQueryRow>`
    select
      ${SiteCheckRuns.id}::text || ':' || coalesce(probe.value->>'region', 'UNKNOWN') as id,
      coalesce(probe.value->>'region', 'UNKNOWN') as region,
      coalesce(probe.value->>'result', ${SiteCheckRuns.availability_result}) as result,
      case
        when coalesce(probe.value->>'status_code', '') ~ '^-?[0-9]+$'
        then (probe.value->>'status_code')::int
        else null
      end as "statusCode",
      case
        when coalesce(probe.value->>'response_time_ms', '') ~ '^-?[0-9]+$'
        then (probe.value->>'response_time_ms')::int
        else null
      end as "responseTimeMs",
      case
        when coalesce(probe.value->>'duration_ms', '') ~ '^-?[0-9]+$'
        then (probe.value->>'duration_ms')::int
        else null
      end as "durationMs",
      coalesce(nullif(probe.value->>'message', ''), ${SiteCheckRuns.error_message}) as message,
      coalesce(nullif(probe.value->>'final_url', ''), ${SiteCheckRuns.final_url}) as "finalUrl",
      case
        when lower(coalesce(probe.value->>'content_verified', '')) = 'true'
        then true
        when lower(coalesce(probe.value->>'content_verified', '')) = 'false'
        then false
        else ${SiteCheckRuns.verify_result} = 'PASSED'
      end as "contentVerified",
      ${SiteCheckRuns.started_time} as "checkTime"
    from ${SiteCheckRuns}
    cross join lateral jsonb_array_elements(
      case
        when jsonb_typeof(${SiteCheckRuns.probe_summary}) = 'array'
          and jsonb_array_length(${SiteCheckRuns.probe_summary}) > 0
        then ${SiteCheckRuns.probe_summary}
        else jsonb_build_array(jsonb_build_object('region', 'UNKNOWN'))
      end
    ) as probe(value)
    where ${SiteCheckRuns.site_id} = ${siteId}
    order by
      ${SiteCheckRuns.started_time} desc,
      case when coalesce(probe.value->>'region', 'UNKNOWN') = 'CN' then 0 else 1 end asc
    limit ${limit}
    offset ${offset}
  `);

  return readQueryRows<SiteCheckQueryRow>(query);
}

async function querySummarySiteChecks(
  app: FastifyInstance,
  siteId: string,
  limit: number,
  offset: number,
): Promise<SiteCheckQueryRow[]> {
  const query = await app.db.read.execute(sql<SiteCheckQueryRow>`
    select
      ${SiteCheckRuns.id}::text || ':SUMMARY' as id,
      case
        when ${SiteCheckRuns.derived_access_scope} = 'CN_ONLY' then 'CN'
        when ${SiteCheckRuns.derived_access_scope} = 'NON_CN_ONLY' then 'GLOBAL'
        when ${SiteCheckRuns.effective_access_scope} = 'CN_ONLY' then 'CN'
        when ${SiteCheckRuns.effective_access_scope} = 'NON_CN_ONLY' then 'GLOBAL'
        else 'UNKNOWN'
      end as region,
      case
        when ${SiteCheckRuns.availability_result} is not null then ${SiteCheckRuns.availability_result}
        when ${SiteCheckRuns.derived_status} = 'OK' then 'SUCCESS'
        when ${SiteCheckRuns.derived_status} = 'WARNING' then 'BLOCKED'
        else 'FAILURE'
      end as result,
      null::int as "statusCode",
      ${SiteCheckRuns.response_time_ms} as "responseTimeMs",
      ${SiteCheckRuns.duration_ms} as "durationMs",
      ${SiteCheckRuns.error_message} as message,
      ${SiteCheckRuns.final_url} as "finalUrl",
      ${SiteCheckRuns.verify_result} = 'PASSED' as "contentVerified",
      ${SiteCheckRuns.started_time} as "checkTime"
    from ${SiteCheckRuns}
    where ${SiteCheckRuns.site_id} = ${siteId}
    order by ${SiteCheckRuns.started_time} desc
    limit ${limit}
    offset ${offset}
  `);

  return readQueryRows<SiteCheckQueryRow>(query);
}

function readQueryRows<T extends object>(value: unknown): T[] {
  if (Array.isArray(value)) {
    return value as T[];
  }

  if (value && typeof value === 'object' && 'rows' in value && Array.isArray(value.rows)) {
    return value.rows as T[];
  }

  return [];
}

function mapPublicSiteCheck(row: SiteCheckQueryRow): PublicSiteCheckItem {
  return {
    id: row.id,
    region: row.region,
    result: row.result,
    statusCode: row.statusCode ?? null,
    responseTimeMs: row.responseTimeMs ?? null,
    durationMs: row.durationMs ?? null,
    message: row.message ?? null,
    finalUrl: row.finalUrl ?? null,
    contentVerified: row.contentVerified === true,
    checkTime: new Date(row.checkTime).toISOString(),
  };
}

import { sql } from 'drizzle-orm';
import {
  check,
  index,
  integer,
  jsonb,
  pgTable,
  text,
  timestamp,
  uuid,
  varchar,
} from 'drizzle-orm/pg-core';
import { v7 } from 'uuid';

import type {
  SiteCheckRegionKey,
  SiteCheckResultKey,
  SiteStatusTypeKey,
} from '../constants/monitoring';
import type { SiteAccessScopeKey } from '../constants/site';
import type {
  ContentValidationStatusKey,
  RunRecordStatusKey,
  SiteCheckModeKey,
  SiteVerifyResultKey,
} from '../constants/task';

import {
  contentValidationStatusEnum,
  runRecordStatusEnum,
  siteAccessScopeEnum,
  siteCheckModeEnum,
  siteCheckResultEnum,
  siteStatusTypeEnum,
  siteVerifyResultEnum,
} from './enums';
import { RequestConfigs } from './request-configs';
import { Sites } from './sites';
import { Jobs } from './tasks';

export interface SiteCheckProbeSummary {
  region: SiteCheckRegionKey;
  result: SiteCheckResultKey;
  summary_level?: SiteStatusTypeKey;
  status_code?: number | null;
  response_time_ms?: number | null;
  duration_ms?: number | null;
  final_url?: string | null;
  content_verified?: boolean;
  page_title?: string | null;
  warning_reason?: string | null;
  raw_result?: string | null;
  message?: string | null;
}

export interface ContentValidationPayload {
  site_title?: string | null;
  rss_feed_url?: string | null;
  sitemap_url?: string | null;
  detected_feed_type?: string | null;
  corrected_fields?: string[];
  proposed_changes?: Record<string, unknown>;
  issues?: string[];
}

export const SiteCheckRuns = pgTable(
  'site_check_runs',
  {
    id: uuid()
      .$default(() => v7())
      .primaryKey(),
    job_id: uuid()
      .notNull()
      .references(() => Jobs.id, {
        onDelete: 'cascade',
        onUpdate: 'cascade',
      }),
    site_id: uuid()
      .notNull()
      .references(() => Sites.id, {
        onDelete: 'cascade',
        onUpdate: 'cascade',
      }),
    request_config_id: uuid().references(() => RequestConfigs.id, {
      onDelete: 'set null',
      onUpdate: 'cascade',
    }),
    status: runRecordStatusEnum().$type<RunRecordStatusKey>().notNull(),
    availability_result: siteCheckResultEnum().$type<SiteCheckResultKey>().notNull(),
    verify_result: siteVerifyResultEnum()
      .$type<SiteVerifyResultKey>()
      .notNull()
      .default('NOT_REQUESTED'),
    effective_access_scope: siteAccessScopeEnum().$type<SiteAccessScopeKey>().notNull(),
    derived_access_scope: siteAccessScopeEnum().$type<SiteAccessScopeKey>().notNull(),
    derived_status: siteStatusTypeEnum().$type<SiteStatusTypeKey>().notNull(),
    check_mode: siteCheckModeEnum().$type<SiteCheckModeKey>().notNull().default('STANDARD'),
    content_validation_status: contentValidationStatusEnum()
      .$type<ContentValidationStatusKey>()
      .notNull()
      .default('NOT_REQUESTED'),
    content_validation_payload: jsonb().$type<ContentValidationPayload>().notNull().default({}),
    probe_summary: jsonb().$type<SiteCheckProbeSummary[]>().notNull().default([]),
    response_time_ms: integer(),
    duration_ms: integer(),
    jitter_ms: integer(),
    final_url: varchar({ length: 512 }),
    error_code: varchar({ length: 64 }),
    error_message: text(),
    started_time: timestamp({ withTimezone: true, precision: 6 }).notNull(),
    finished_time: timestamp({ withTimezone: true, precision: 6 }),
    created_time: timestamp({ withTimezone: true, precision: 6 }).notNull().defaultNow(),
  },
  (table) => [
    index('site_check_runs_job_id_started_time_index').on(table.job_id, table.started_time.desc()),
    index('site_check_runs_site_id_started_time_index').on(
      table.site_id,
      table.started_time.desc(),
    ),
    index('site_check_runs_site_id_status_started_time_index').on(
      table.site_id,
      table.status,
      table.started_time.desc(),
    ),
    check(
      'site_check_runs_response_time_non_negative_check',
      sql`${table.response_time_ms} is null or ${table.response_time_ms} >= 0`,
    ),
    check(
      'site_check_runs_duration_non_negative_check',
      sql`${table.duration_ms} is null or ${table.duration_ms} >= 0`,
    ),
    check(
      'site_check_runs_jitter_non_negative_check',
      sql`${table.jitter_ms} is null or ${table.jitter_ms} >= 0`,
    ),
  ],
);

export const SiteChecks = SiteCheckRuns;

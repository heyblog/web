import { sql } from 'drizzle-orm';
import {
  boolean,
  integer,
  jsonb,
  pgView,
  text,
  timestamp,
  uuid,
  varchar,
} from 'drizzle-orm/pg-core';

import type { MultiFeed } from './sites';

/** 标签使用统计视图 */
export const TagStats = pgView('tag_stats', {
  /** 标签 ID */
  tag_id: uuid('tag_id'),
  /** 标签名称 */
  tag_name: varchar('tag_name', { length: 64 }),
  /** 标签类型 */
  tag_type: varchar('tag_type', { length: 32 }),
  /** 关联站点数量 */
  site_count: integer('site_count'),
}).as(sql`
  select
    td.id as tag_id,
    td.name as tag_name,
    td.tag_type as tag_type,
    count(st.site_id)::int as site_count
  from tag_definitions td
  left join site_tags st on st.tag_id = td.id
  group by td.id, td.name, td.tag_type
`);

/** 技术架构使用统计视图 */
export const TechnologyStats = pgView('technology_stats', {
  /** 技术项 ID */
  technology_id: uuid('technology_id'),
  /** 技术名称 */
  technology_name: varchar('technology_name', { length: 128 }),
  /** 技术类型 */
  technology_type: varchar('technology_type', { length: 32 }),
  /** 被站点引用次数 */
  site_count: integer('site_count'),
}).as(sql`
  with technology_refs as (
    select distinct
      sa.site_id,
      pts.catalog_id as technology_id
    from site_architectures sa
    inner join program_technology_stacks pts on pts.program_id = sa.program_id
    where pts.catalog_id is not null
  )
  select
    tc.id as technology_id,
    tc.name as technology_name,
    tc.technology_type as technology_type,
    count(tr.site_id)::int as site_count
  from technology_catalogs tc
  left join technology_refs tr on tr.technology_id = tc.id
  group by tc.id, tc.name, tc.technology_type
`);

/** 站点检测统计视图 */
export const SiteCheckStats = pgView('site_check_stats', {
  /** 站点 ID */
  site_id: uuid('site_id'),
  /** 检测总次数 */
  total_checks: integer('total_checks'),
  /** 成功检测次数 */
  success_checks: integer('success_checks'),
  /** 失败检测次数 */
  failed_checks: integer('failed_checks'),
  /** 平均响应时间 */
  avg_response_time_ms: integer('avg_response_time_ms'),
}).as(sql`
  select
    scr.site_id as site_id,
    count(*)::int as total_checks,
    count(*) filter (where scr.derived_status = 'OK')::int as success_checks,
    count(*) filter (where scr.derived_status <> 'OK')::int as failed_checks,
    avg(scr.response_time_ms)::int as avg_response_time_ms
  from site_check_runs scr
  group by scr.site_id
`);

/** 站点最新一次检测结果视图 */
export const LatestSiteChecks = pgView('latest_site_checks', {
  /** 站点 ID */
  site_id: uuid('site_id'),
  /** 最近一次检测记录 ID */
  check_id: uuid('check_id'),
  /** 最近一次检测地域 */
  region: varchar('region', { length: 32 }),
  /** 最近一次检测结果 */
  result: varchar('result', { length: 32 }),
  /** 最近一次 HTTP 状态码 */
  status_code: integer('status_code'),
  /** 最近一次响应耗时 */
  response_time_ms: integer('response_time_ms'),
  /** 最近一次总耗时 */
  duration_ms: integer('duration_ms'),
  /** 最终跳转地址 */
  final_url: varchar('final_url', { length: 256 }),
  /** 内容是否通过校验 */
  content_verified: boolean('content_verified'),
  /** 最近一次检测时间 */
  check_time: timestamp('check_time', { withTimezone: true, precision: 6 }),
}).as(sql`
  select distinct on (scr.site_id)
    scr.site_id as site_id,
    scr.id as check_id,
    'UNKNOWN'::varchar as region,
    scr.derived_status as result,
    null::integer as status_code,
    scr.response_time_ms as response_time_ms,
    scr.duration_ms as duration_ms,
    scr.final_url as final_url,
    (scr.verify_result = 'PASSED') as content_verified,
    scr.started_time as check_time
  from site_check_runs scr
  order by scr.site_id, scr.started_time desc, scr.id desc
`);

/** 站点文章聚合统计视图 */
export const SiteFeedArticleStats = pgView('site_feed_article_stats', {
  /** 站点 ID */
  site_id: uuid('site_id'),
  /** 抓取文章总数 */
  total_articles: integer('total_articles'),
  /** 当前可见文章数 */
  visible_articles: integer('visible_articles'),
  /** 最近一次抓取时间 */
  latest_fetched_time: timestamp('latest_fetched_time', {
    withTimezone: true,
    precision: 6,
  }),
  /** 最近一篇文章发布时间 */
  latest_published_time: timestamp('latest_published_time', {
    withTimezone: true,
    precision: 6,
  }),
}).as(sql`
  select
    fa.site_id as site_id,
    count(*)::int as total_articles,
    count(*) filter (where fa.visibility = 'VISIBLE')::int as visible_articles,
    max(fa.fetched_time) as latest_fetched_time,
    max(fa.published_time) as latest_published_time
  from feed_articles fa
  group by fa.site_id
`);

/** 站点访问计数聚合视图 */
export const SiteAccessCounters = pgView('site_access_counters', {
  /** 站点 ID */
  site_id: uuid('site_id'),
  /** 访问总次数 */
  total: integer('total'),
  /** 最近一次访问时间 */
  updated_time: timestamp('updated_time', {
    withTimezone: true,
    precision: 6,
  }),
}).as(sql`
  select
    s.id as site_id,
    count(sae.id)::int as total,
    max(sae.occurred_time) as updated_time
  from sites s
  left join site_access_events sae on sae.site_id = s.id
  group by s.id
`);

/** 站点访问来源聚合视图 */
export const SiteAccessSourceStats = pgView('site_access_source_stats', {
  /** 站点 ID */
  site_id: uuid('site_id'),
  /** 访问事件类型 */
  event_type: varchar('event_type', { length: 64 }),
  /** 访问来源 */
  source: varchar('source', { length: 64 }),
  /** 该来源访问次数 */
  total: integer('total'),
  /** 最近一次访问时间 */
  latest_access_time: timestamp('latest_access_time', {
    withTimezone: true,
    precision: 6,
  }),
}).as(sql`
  select
    sae.site_id as site_id,
    sae.event_type as event_type,
    sae.source as source,
    count(*)::int as total,
    max(sae.occurred_time) as latest_access_time
  from site_access_events sae
  group by sae.site_id, sae.event_type, sae.source
`);

/** 站点按访问事件类型聚合统计视图 */
export const SiteAccessEventTypeStats = pgView('site_access_event_type_stats', {
  /** 站点 ID */
  site_id: uuid('site_id'),
  /** 访问事件类型 */
  event_type: varchar('event_type', { length: 64 }),
  /** 该事件类型访问次数 */
  total: integer('total'),
  /** 最近一次访问时间 */
  latest_access_time: timestamp('latest_access_time', {
    withTimezone: true,
    precision: 6,
  }),
}).as(sql`
  select
    sae.site_id as site_id,
    sae.event_type as event_type,
    count(*)::int as total,
    max(sae.occurred_time) as latest_access_time
  from site_access_events sae
  group by sae.site_id, sae.event_type
`);

/** 站点警示标签统计视图 */
export const SiteWarningTagStats = pgView('site_warning_tag_stats', {
  /** 警示标签定义 ID */
  tag_id: uuid('tag_id'),
  /** 警示标签机器键 */
  machine_key: varchar('machine_key', { length: 64 }),
  /** 警示标签展示名称 */
  tag_name: varchar('tag_name', { length: 64 }),
  /** 带有该标签的站点数量 */
  site_count: integer('site_count'),
}).as(sql`
  select
    td.id as tag_id,
    td.machine_key as machine_key,
    td.name as tag_name,
    count(distinct swt.site_id)::int as site_count
  from site_warning_tags swt
  inner join tag_definitions td on td.id = swt.tag_id
  group by td.id, td.machine_key, td.name
`);

/** 站点标签聚合视图 */
export const SiteTagSummaryView = pgView('site_tag_summary_view', {
  /** 站点 ID */
  site_id: uuid('site_id'),
  /** 主标签名称 */
  primary_tag: text('primary_tag'),
  /** 副标签名称列表 */
  sub_tags: text('sub_tags').array(),
}).as(sql`
  select
    st.site_id as site_id,
    min(td.name) filter (where td.tag_type = 'MAIN') as primary_tag,
    coalesce(
      array_agg(distinct td.name order by td.name) filter (where td.tag_type = 'SUB'),
      '{}'::text[]
    ) as sub_tags
  from site_tags st
  inner join tag_definitions td on td.id = st.tag_id
  where td.is_enabled = true
  group by st.site_id
`);

/** 站点警示标签聚合视图 */
export const SiteWarningTagSummaryView = pgView('site_warning_tag_summary_view', {
  /** 站点 ID */
  site_id: uuid('site_id'),
  /** 警示标签数组 */
  warning_tags: jsonb('warning_tags').$type<
    Array<{
      machineKey: string;
      name: string;
      description: string | null;
    }>
  >(),
  /** 警示标签名称列表 */
  warning_names: text('warning_names').array(),
}).as(sql`
  with warning_rows as (
    select distinct
      swt.site_id as site_id,
      td.machine_key as machine_key,
      td.name as name,
      td.description as description
    from site_warning_tags swt
    inner join tag_definitions td on td.id = swt.tag_id
    where td.tag_type = 'WARNING'
      and td.is_enabled = true
  )
  select
    warning_rows.site_id as site_id,
    coalesce(
      jsonb_agg(
        jsonb_build_object(
          'machineKey',
          warning_rows.machine_key,
          'name',
          warning_rows.name,
          'description',
          warning_rows.description
        )
        order by warning_rows.name
      ),
      '[]'::jsonb
    ) as warning_tags,
    coalesce(array_agg(warning_rows.name order by warning_rows.name), '{}'::text[]) as warning_names
  from warning_rows
  group by warning_rows.site_id
`);

/** 站点程序聚合视图 */
export const SiteProgramSummaryView = pgView('site_program_summary_view', {
  /** 站点 ID */
  site_id: uuid('site_id'),
  /** 程序 ID */
  program_id: uuid('program_id'),
  /** 程序名称 */
  program_name: varchar('program_name', { length: 128 }),
  /** 是否开源 */
  program_is_open_source: boolean('program_is_open_source'),
  /** 程序官网 */
  website_url: varchar('website_url', { length: 512 }),
  /** 程序仓库 */
  repo_url: varchar('repo_url', { length: 512 }),
}).as(sql`
  select
    sa.site_id as site_id,
    p.id as program_id,
    p.name as program_name,
    p.is_open_source as program_is_open_source,
    p.website_url as website_url,
    p.repo_url as repo_url
  from site_architectures sa
  inner join programs p on p.id = sa.program_id
  where p.is_enabled = true
`);

/** 公共站点目录读模型视图 */
export const PublicSiteDirectoryReadView = pgView('public_site_directory_read_view', {
  /** 站点 ID */
  site_id: uuid('site_id'),
  /** 外部业务 ID */
  bid: varchar('bid', { length: 64 }),
  /** 站点名称 */
  name: varchar('name', { length: 64 }),
  /** 站点 URL */
  url: varchar('url', { length: 256 }),
  /** 站点签名 */
  sign: text('sign'),
  /** 原始订阅源列表 */
  feeds: jsonb('feeds').$type<MultiFeed[]>(),
  /** 默认订阅源 URL */
  feed_url: varchar('feed_url', { length: 512 }),
  /** 地图地址 */
  sitemap: varchar('sitemap', { length: 256 }),
  /** 友链页地址 */
  link_page: varchar('link_page', { length: 256 }),
  /** 是否推荐 */
  featured: boolean('featured'),
  /** 站点状态 */
  status: varchar('status', { length: 32 }),
  /** 访问范围 */
  access_scope: varchar('access_scope', { length: 32 }),
  /** 加入时间 */
  join_time: timestamp('join_time', { withTimezone: true, precision: 6 }),
  /** 更新时间 */
  update_time: timestamp('update_time', { withTimezone: true, precision: 6 }),
  /** 备注原因 */
  reason: text('reason'),
  /** 文章数量 */
  article_count: integer('article_count'),
  /** 最近内容发布时间 */
  latest_published_time: timestamp('latest_published_time', {
    withTimezone: true,
    precision: 6,
  }),
  /** 访问次数 */
  visit_count: integer('visit_count'),
  /** 主标签 */
  primary_tag: text('primary_tag'),
  /** 副标签 */
  sub_tags: text('sub_tags').array(),
  /** 警示标签 */
  warning_tags: jsonb('warning_tags').$type<
    Array<{
      machineKey: string;
      name: string;
      description: string | null;
    }>
  >(),
  /** 警示标签名称 */
  warning_names: text('warning_names').array(),
  /** 程序 ID */
  program_id: uuid('program_id'),
  /** 程序名称 */
  program_name: varchar('program_name', { length: 128 }),
  /** 是否开源 */
  program_is_open_source: boolean('program_is_open_source'),
  /** 程序官网 */
  website_url: varchar('website_url', { length: 512 }),
  /** 程序仓库 */
  repo_url: varchar('repo_url', { length: 512 }),
}).as(sql`
  select
    s.id as site_id,
    s.bid as bid,
    s.name as name,
    s.url as url,
    coalesce(s.sign, '') as sign,
    s.feed as feeds,
    (
      select feed_item->>'url'
      from jsonb_array_elements(s.feed) as feed_item
      where coalesce(feed_item->>'url', '') <> ''
        and coalesce((feed_item->>'isDefault')::boolean, false)
      limit 1
    ) as feed_url,
    s.sitemap as sitemap,
    s.link_page as link_page,
    s.recommend as featured,
    s.status as status,
    s.access_scope as access_scope,
    s.join_time as join_time,
    s.update_time as update_time,
    s.reason as reason,
    coalesce(article_stats.visible_articles, article_stats.total_articles, 0)::int as article_count,
    article_stats.latest_published_time as latest_published_time,
    coalesce(access_counters.total, 0)::int as visit_count,
    tag_summary.primary_tag as primary_tag,
    coalesce(tag_summary.sub_tags, '{}'::text[]) as sub_tags,
    coalesce(warning_summary.warning_tags, '[]'::jsonb) as warning_tags,
    coalesce(warning_summary.warning_names, '{}'::text[]) as warning_names,
    program_summary.program_id as program_id,
    program_summary.program_name as program_name,
    program_summary.program_is_open_source as program_is_open_source,
    program_summary.website_url as website_url,
    program_summary.repo_url as repo_url
  from sites s
  left join (
    select
      fa.site_id as site_id,
      count(*)::int as total_articles,
      count(*) filter (where fa.visibility = 'VISIBLE')::int as visible_articles,
      max(fa.published_time) as latest_published_time
    from feed_articles fa
    group by fa.site_id
  ) article_stats on article_stats.site_id = s.id
  left join (
    select
      sae.site_id as site_id,
      count(*)::int as total
    from site_access_events sae
    group by sae.site_id
  ) access_counters on access_counters.site_id = s.id
  left join (
    select
      st.site_id as site_id,
      min(td.name) filter (where td.tag_type = 'MAIN') as primary_tag,
      coalesce(
        array_agg(distinct td.name order by td.name) filter (where td.tag_type = 'SUB'),
        '{}'::text[]
      ) as sub_tags
    from site_tags st
    inner join tag_definitions td on td.id = st.tag_id
    where td.is_enabled = true
    group by st.site_id
  ) tag_summary on tag_summary.site_id = s.id
  left join (
    with warning_rows as (
      select distinct
        swt.site_id as site_id,
        td.machine_key as machine_key,
        td.name as name,
        td.description as description
      from site_warning_tags swt
      inner join tag_definitions td on td.id = swt.tag_id
      where td.tag_type = 'WARNING'
        and td.is_enabled = true
    )
    select
      warning_rows.site_id as site_id,
      coalesce(
        jsonb_agg(
          jsonb_build_object(
            'machineKey',
            warning_rows.machine_key,
            'name',
            warning_rows.name,
            'description',
            warning_rows.description
          )
          order by warning_rows.name
        ),
        '[]'::jsonb
      ) as warning_tags,
      coalesce(array_agg(warning_rows.name order by warning_rows.name), '{}'::text[]) as warning_names
    from warning_rows
    group by warning_rows.site_id
  ) warning_summary on warning_summary.site_id = s.id
  left join (
    select
      sa.site_id as site_id,
      p.id as program_id,
      p.name as program_name,
      p.is_open_source as program_is_open_source,
      p.website_url as website_url,
      p.repo_url as repo_url
    from site_architectures sa
    inner join programs p on p.id = sa.program_id
    where p.is_enabled = true
  ) program_summary on program_summary.site_id = s.id
  where s.is_show = true
`);

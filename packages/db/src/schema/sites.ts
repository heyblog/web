import { sql } from 'drizzle-orm';
import {
  boolean,
  check,
  index,
  jsonb,
  pgTable,
  primaryKey,
  text,
  timestamp,
  uniqueIndex,
  uuid,
  varchar,
} from 'drizzle-orm/pg-core';
import { v7 } from 'uuid';

import type { FeedTypeKey } from '../constants/site';

import { TagDefinitions, TechnologyCatalogs } from './catalogs';
import {
  fromSources,
  siteAccessEventTypeEnum,
  siteAccessScopeEnum,
  siteClassificationStatusEnum,
  siteStatusTypeEnum,
  siteTechStackCategoryEnum,
} from './enums';

export interface MultiFeed {
  name: string;
  url: string;
  type?: FeedTypeKey;
  isDefault: boolean;
}

export const Sites = pgTable(
  'sites',
  {
    /** 站点主键 */
    id: uuid()
      .$default(() => v7())
      .primaryKey(),
    /** 面向外部展示与管理的站点唯一业务标识 */
    bid: varchar({ length: 64 }).unique(),
    /** 站点名称 */
    name: varchar({ length: 64 }).notNull(),
    /** 站点主页地址 */
    url: varchar({ length: 256 }).unique().notNull(),
    /** 站点签名或简短描述 */
    sign: text().default(''),
    /** 站点图标的 base64 编码字符串 */
    icon_base64: text(),
    /** 站点订阅源列表 */
    feed: jsonb().$type<MultiFeed[]>().notNull().default([]),
    /** 站点来源渠道 */
    from: fromSources().array().notNull().default([]),
    /** 站点分类信息是否已确认完整 */
    classification_status: siteClassificationStatusEnum().notNull().default('COMPLETE'),
    /** 站点地图地址 */
    sitemap: varchar({ length: 256 }),
    /** 友链页地址 */
    link_page: varchar({ length: 256 }),
    /** 站点加入时间 */
    join_time: timestamp({ withTimezone: true, precision: 6 })
      .notNull()
      .$default(() => new Date()),
    /** 站点最后更新时间 */
    update_time: timestamp({ withTimezone: true, precision: 6 })
      .notNull()
      .$default(() => new Date()),
    /** 站点访问属性：仅国内、仅海外、国内外都可访问 */
    access_scope: siteAccessScopeEnum().notNull().default('ALL'),
    /** 站点当前简易显示状态，取最近一次检测归并结果 */
    status: siteStatusTypeEnum().notNull().default('OK'),
    /** 是否在前台显示 */
    is_show: boolean().notNull().default(true),
    /** 是否推荐 */
    recommend: boolean().notNull().default(false),
    /** 不显示或异常时的补充原因 */
    reason: text(),
  },
  (table) => [
    index('sites_from_gin_index').using('gin', table.from),
    index('sites_name_index').on(table.name),
    index('sites_name_lower_index').on(sql`lower(${table.name})`),
    index('sites_access_scope_index').on(table.access_scope),
    index('sites_status_index').on(table.status),
    index('sites_is_show_index').on(table.is_show),
    index('sites_recommend_index').on(table.recommend),
    index('sites_join_time_index').on(table.join_time.desc()),
    index('sites_update_time_index').on(table.update_time.desc()),
    check('sites_bid_not_blank_check', sql`btrim(${table.bid}) <> ''`),
    check('sites_name_not_blank_check', sql`btrim(${table.name}) <> ''`),
  ],
);

/** 程序定义表：用于站点主程序归档、统计与展示 */
export const Programs = pgTable(
  'programs',
  {
    id: uuid()
      .$default(() => v7())
      .primaryKey(),
    /** 程序展示名称 */
    name: varchar({ length: 128 }).notNull(),
    /** 程序名称归一化键，用于查重与模糊检索 */
    name_normalized: varchar({ length: 128 }).notNull(),
    /** 程序是否开源 */
    is_open_source: boolean().notNull().default(false),
    /** 程序官网或主页 */
    website_url: varchar({ length: 512 }),
    /** 程序仓库地址 */
    repo_url: varchar({ length: 512 }),
    /** 是否启用 */
    is_enabled: boolean().notNull().default(true),
    /** 创建时间 */
    created_time: timestamp({ withTimezone: true, precision: 6 }).notNull().defaultNow(),
    /** 更新时间 */
    updated_time: timestamp({ withTimezone: true, precision: 6 })
      .notNull()
      .defaultNow()
      .$onUpdateFn(() => new Date()),
  },
  (table) => [
    uniqueIndex('programs_name_normalized_unique').on(table.name_normalized),
    index('programs_enabled_name_index').on(table.is_enabled, table.name),
    check('programs_name_not_blank_check', sql`btrim(${table.name}) <> ''`),
    check('programs_name_normalized_not_blank_check', sql`btrim(${table.name_normalized}) <> ''`),
  ],
);

/** 站点标签关联表，通过标签定义表表达主标签和副标签 */
export const SiteTags = pgTable(
  'site_tags',
  {
    /** 关联站点 */
    site_id: uuid()
      .notNull()
      .references(() => Sites.id, {
        onDelete: 'cascade',
        onUpdate: 'cascade',
      }),
    /** 关联标签 */
    tag_id: uuid()
      .notNull()
      .references(() => TagDefinitions.id, {
        onDelete: 'cascade',
        onUpdate: 'cascade',
      }),
    /** 关联创建时间 */
    created_time: timestamp({ withTimezone: true, precision: 6 }).notNull().defaultNow(),
  },
  (table) => [
    primaryKey({
      name: 'site_tags_pkey',
      columns: [table.site_id, table.tag_id],
    }),
    index('site_tags_site_id_index').on(table.site_id),
    index('site_tags_tag_id_index').on(table.tag_id),
  ],
);

/** 站点程序关联表：站点与程序一对一关联 */
export const SiteArchitectures = pgTable(
  'site_architectures',
  {
    /** 站点 ID，与站点表一对一 */
    site_id: uuid()
      .primaryKey()
      .references(() => Sites.id, {
        onDelete: 'cascade',
        onUpdate: 'cascade',
      }),
    /** 博客程序（程序表项） */
    program_id: uuid()
      .notNull()
      .references(() => Programs.id, {
        onDelete: 'restrict',
        onUpdate: 'cascade',
      }),
    /** 创建时间 */
    created_time: timestamp({ withTimezone: true, precision: 6 }).notNull().defaultNow(),
    /** 更新时间 */
    updated_time: timestamp({ withTimezone: true, precision: 6 })
      .notNull()
      .defaultNow()
      .$onUpdateFn(() => new Date()),
  },
  (table) => [index('site_architectures_program_id_index').on(table.program_id)],
);

/** 程序技术栈关联表：程序与技术栈候选词库的一对多关系 */
export const ProgramTechnologyStacks = pgTable(
  'program_technology_stacks',
  {
    id: uuid()
      .$default(() => v7())
      .primaryKey(),
    /** 关联程序 */
    program_id: uuid()
      .notNull()
      .references(() => Programs.id, {
        onDelete: 'cascade',
        onUpdate: 'cascade',
      }),
    /** 技术目录引用，不在目录中的自定义项可为空 */
    catalog_id: uuid().references(() => TechnologyCatalogs.id, {
      onDelete: 'set null',
      onUpdate: 'cascade',
    }),
    /** 技术项分类 */
    category: siteTechStackCategoryEnum().notNull(),
    /** 自定义展示名称，目录命中时可为空 */
    name_custom: varchar({ length: 128 }),
    /** 归一化名称，用于去重与检索 */
    name_normalized: varchar({ length: 128 }).notNull(),
    /** 创建时间 */
    created_time: timestamp({ withTimezone: true, precision: 6 }).notNull().defaultNow(),
    /** 更新时间 */
    updated_time: timestamp({ withTimezone: true, precision: 6 })
      .notNull()
      .defaultNow()
      .$onUpdateFn(() => new Date()),
  },
  (table) => [
    index('program_technology_stacks_program_id_index').on(table.program_id),
    index('program_technology_stacks_catalog_id_index').on(table.catalog_id),
    index('program_technology_stacks_category_index').on(table.category),
    index('program_technology_stacks_name_normalized_index').on(table.name_normalized),
    uniqueIndex('program_technology_stacks_program_category_name_unique').on(
      table.program_id,
      table.category,
      table.name_normalized,
    ),
    check(
      'program_technology_stacks_name_normalized_not_blank_check',
      sql`btrim(${table.name_normalized}) <> ''`,
    ),
  ],
);

/** 站点访问事件表，记录每次访问来源与入口信息 */
export const SiteAccessEvents = pgTable(
  'site_access_events',
  {
    /** 访问事件主键 */
    id: uuid()
      .$default(() => v7())
      .primaryKey(),
    /** 被访问的站点 */
    site_id: uuid()
      .notNull()
      .references(() => Sites.id, {
        onDelete: 'cascade',
        onUpdate: 'cascade',
      }),
    /** 事件类型：默认记录本项目的跳转点击，也可记录嵌入脚本上报的访问 */
    event_type: siteAccessEventTypeEnum().notNull().default('OUTBOUND_CLICK'),
    /** 可选来源渠道标识，例如 direct / search / internal / embed */
    source: varchar({ length: 64 }).notNull().default('UNKNOWN'),
    /** 来源域名或来源主机 */
    referer_host: varchar({ length: 256 }),
    /** 访问页面路径：跳转点击时为本项目页面路径，嵌入访问时为源站页面路径 */
    path: varchar({ length: 512 }),
    /** 访问终端标识 */
    user_agent: varchar({ length: 512 }),
    /** 事件发生时间 */
    occurred_time: timestamp({ withTimezone: true, precision: 6 }).notNull().defaultNow(),
  },
  (table) => [
    index('site_access_events_site_id_occurred_time_index').on(
      table.site_id,
      table.occurred_time.desc(),
    ),
    index('site_access_events_site_id_event_type_occurred_time_index').on(
      table.site_id,
      table.event_type,
      table.occurred_time.desc(),
    ),
    index('site_access_events_site_id_event_type_source_occurred_time_index').on(
      table.site_id,
      table.event_type,
      table.source,
      table.occurred_time.desc(),
    ),
    index('site_access_events_event_type_occurred_time_index').on(
      table.event_type,
      table.occurred_time.desc(),
    ),
    index('site_access_events_referer_host_index').on(table.referer_host),
    index('site_access_events_occurred_time_index').on(table.occurred_time.desc()),
    check('site_access_events_source_not_blank_check', sql`btrim(${table.source}) <> ''`),
  ],
);

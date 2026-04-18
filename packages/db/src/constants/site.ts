export const MAIN_TAGS = {
  LIFE: {
    label: '生活',
    description: '以生活记录、日常分享、随笔表达为主的博客内容分类',
  },
  TECH: {
    label: '技术',
    description: '以编程、软件工程、系统运维与技术研究为主的博客内容分类',
  },
  KNOWLEDGE: {
    label: '知识',
    description: '以知识整理、学习笔记、方法论总结为主的博客内容分类',
  },
  CURATION: {
    label: '整合',
    description: '以资料整理、资源聚合、专题汇总为主的博客内容分类',
  },
  COLLECTION: {
    label: '采集',
    description: '以自动采集、信息抓取、内容汇编为主的博客内容分类',
  },
  GENERAL: {
    label: '综合',
    description: '内容主题较为综合，无法归为单一垂类的博客内容分类',
  },
} as const;

export const MAIN_TAG_KEYS = Object.keys(MAIN_TAGS) as Array<keyof typeof MAIN_TAGS>;

export type MainTagKey = (typeof MAIN_TAG_KEYS)[number];

export const FROM_SOURCES = {
  CIB: {
    label: '中文独立博客列表',
    description: '来自中文独立博客列表项目的数据同步',
  },
  BO_YOU_QUAN: {
    label: '博友圈',
    description: '来自博友圈项目的数据同步',
  },
  BLOG_FINDER: {
    label: 'BlogFinder',
    description: '来自 BlogFinder 项目的数据同步',
  },
  BKZ: {
    label: '优秀个人独立博客导航',
    description: '来自优秀个人独立博客导航项目的数据同步',
  },
  TRAVELLINGS: {
    label: '开往',
    description: '来自开往项目的数据同步',
  },
  WEB_SUBMIT: {
    label: '网页提交',
    description: '由用户通过网页表单主动提交',
  },
  LINK_PAGE_SEARCH: {
    label: '友链发现',
    description: '通过站点友链页面自动发现并录入',
  },
  OLD_DATA: {
    label: '旧版数据迁移',
    description: '从历史系统或旧数据结构迁移而来',
  },
} as const;

export const FROM_SOURCE_KEYS = Object.keys(FROM_SOURCES) as Array<keyof typeof FROM_SOURCES>;

export type FromSourceKey = (typeof FROM_SOURCE_KEYS)[number];

export const FEED_TYPES = {
  RSS: {
    label: 'RSS',
    description: '使用 RSS 规范提供的订阅源',
  },
  ATOM: {
    label: 'Atom',
    description: '使用 Atom 规范提供的订阅源',
  },
  JSON: {
    label: 'JSON Feed',
    description: '使用 JSON Feed 规范提供的订阅源',
  },
} as const;

export const FEED_TYPE_KEYS = Object.keys(FEED_TYPES) as Array<keyof typeof FEED_TYPES>;

export type FeedTypeKey = (typeof FEED_TYPE_KEYS)[number];

export const SITE_CLASSIFICATION_STATUSES = {
  COMPLETE: {
    label: '分类完整',
    description: '主标签已确认，无需额外补充分类型信息',
  },
  NEEDS_REVIEW: {
    label: '待完善分类',
    description: '主标签使用综合兜底，仍需用户或管理员补充分类型信息',
  },
} as const;

export const SITE_CLASSIFICATION_STATUS_KEYS = Object.keys(SITE_CLASSIFICATION_STATUSES) as Array<
  keyof typeof SITE_CLASSIFICATION_STATUSES
>;

export type SiteClassificationStatusKey = (typeof SITE_CLASSIFICATION_STATUS_KEYS)[number];

export const SITE_ACCESS_EVENT_TYPES = {
  OUTBOUND_CLICK: {
    label: '跳转点击',
    description: '用户在本项目页面点击目标站点链接并跳转的访问事件',
  },
  EMBED_PAGEVIEW: {
    label: '嵌入访问',
    description: '站长在源站完成嵌入统计脚本配置后上报的页面访问事件',
  },
} as const;

export const SITE_ACCESS_EVENT_TYPE_KEYS = Object.keys(SITE_ACCESS_EVENT_TYPES) as Array<
  keyof typeof SITE_ACCESS_EVENT_TYPES
>;

export type SiteAccessEventTypeKey = (typeof SITE_ACCESS_EVENT_TYPE_KEYS)[number];

export const SITE_ACCESS_SCOPES = {
  CN_ONLY: {
    label: '仅国内可访问',
    description: '站点主要面向国内网络环境访问，海外访问可能受限',
  },
  NON_CN_ONLY: {
    label: '仅海外可访问',
    description: '站点仅在非中国大陆网络环境下可访问',
  },
  ALL: {
    label: '国内外均可访问',
    description: '站点在国内与海外网络环境下都可正常访问',
  },
} as const;

export const SITE_ACCESS_SCOPE_KEYS = Object.keys(SITE_ACCESS_SCOPES) as Array<
  keyof typeof SITE_ACCESS_SCOPES
>;

export type SiteAccessScopeKey = (typeof SITE_ACCESS_SCOPE_KEYS)[number];

export const SITE_CLAIM_TYPES = {
  OWNER: {
    label: '所有者认领',
    description: '由站点所有者发起并通过验证流程完成认领',
  },
  ADMIN: {
    label: '管理员认证',
    description: '由管理员审核确认后建立站点归属关系',
  },
} as const;

export const SITE_CLAIM_TYPE_KEYS = Object.keys(SITE_CLAIM_TYPES) as Array<
  keyof typeof SITE_CLAIM_TYPES
>;

export type SiteClaimTypeKey = (typeof SITE_CLAIM_TYPE_KEYS)[number];

export const SITE_CLAIM_STATUSES = {
  PENDING_VERIFICATION: {
    label: '待验证',
    description: '认领申请已提交，等待提交者完成所有权验证',
  },
  PENDING_REVIEW: {
    label: '待审核',
    description: '验证材料已提交，等待管理员审核',
  },
  APPROVED: {
    label: '已通过',
    description: '认领申请通过，站点归属关系已建立',
  },
  REJECTED: {
    label: '已拒绝',
    description: '认领申请被拒绝，需要重新发起',
  },
  CANCELED: {
    label: '已撤销',
    description: '申请人撤销或系统终止了本次认领流程',
  },
} as const;

export const SITE_CLAIM_STATUS_KEYS = Object.keys(SITE_CLAIM_STATUSES) as Array<
  keyof typeof SITE_CLAIM_STATUSES
>;

export type SiteClaimStatusKey = (typeof SITE_CLAIM_STATUS_KEYS)[number];

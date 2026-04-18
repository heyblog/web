export const SITE_CHECK_REGIONS = {
  CN: {
    label: '国内',
    description: '由国内网络环境发起的检测',
  },
  GLOBAL: {
    label: '海外',
    description: '由海外或全球边缘节点发起的检测',
  },
  UNKNOWN: {
    label: '未知',
    description: '无法识别或未标注检测来源地域',
  },
} as const;

export const SITE_CHECK_REGION_KEYS = Object.keys(SITE_CHECK_REGIONS) as Array<
  keyof typeof SITE_CHECK_REGIONS
>;

export type SiteCheckRegionKey = (typeof SITE_CHECK_REGION_KEYS)[number];

export const SITE_CHECK_RESULTS = {
  SUCCESS: {
    label: '访问成功',
    description: '站点请求成功并通过可用性判定',
  },
  FAILURE: {
    label: '访问失败',
    description: '站点请求失败或不可达',
  },
  TIMEOUT: {
    label: '访问超时',
    description: '站点请求超过超时时间仍未完成',
  },
  DNS_ERROR: {
    label: 'DNS 异常',
    description: '域名解析失败或 DNS 响应异常',
  },
  SSL_ERROR: {
    label: 'SSL 异常',
    description: '站点证书错误或 TLS 握手失败',
  },
  HTTP_ERROR: {
    label: 'HTTP 异常',
    description: '站点返回异常状态码或错误页面',
  },
  BLOCKED: {
    label: '访问受限',
    description: '站点受区域、风控或权限限制而不可访问',
  },
} as const;

export const SITE_CHECK_RESULT_KEYS = Object.keys(SITE_CHECK_RESULTS) as Array<
  keyof typeof SITE_CHECK_RESULTS
>;

export type SiteCheckResultKey = (typeof SITE_CHECK_RESULT_KEYS)[number];

export const SITE_STATUS_TYPES = {
  OK: {
    label: '状态正常',
    description: '网站整体可访问，前台展示为正常状态',
  },
  WARNING: {
    label: '状态警告',
    description: '网站部分场景受限、降级或存在风险，需要关注',
  },
  ERROR: {
    label: '状态异常',
    description: '网站不可访问、返回错误或整体状态异常',
  },
} as const;

export const SITE_STATUS_TYPE_KEYS = Object.keys(SITE_STATUS_TYPES) as Array<
  keyof typeof SITE_STATUS_TYPES
>;

export type SiteStatusTypeKey = (typeof SITE_STATUS_TYPE_KEYS)[number];

export const SITE_WARNING_TAGS = {
  EXTERNAL_LIMIT: {
    label: '外部限制',
    description: '外部地区无法正常访问',
  },
  INTERNAL_LIMIT: {
    label: '内部限制',
    description: '内部地区无法正常访问',
  },
  FEW_ARTICLES: {
    label: '文章较少',
    description: '站点内容量较小，文章数量明显偏少',
  },
  NO_CONTENT: {
    label: '无内容',
    description: '网站可访问但缺少有效内容或页面为空',
  },
  NON_ORIGINAL: {
    label: '非原创',
    description: '站点内容以转载、聚合或搬运为主',
  },
  SENSITIVE_CONTENT: {
    label: '敏感内容',
    description: '站点存在敏感或不适宜公开展示的内容风险',
  },
} as const;

export type SiteWarningTagMachineKey = keyof typeof SITE_WARNING_TAGS;

export const SITE_WARNING_TAG_SOURCES = {
  ARTICLE_FEEDBACK: {
    label: '文章反馈',
    description: '由文章反馈审核通过后同步给站点打上的警示标签',
  },
  SITE_FEEDBACK: {
    label: '站点反馈',
    description: '由站点反馈审核通过后同步给站点打上的警示标签',
  },
  MANUAL: {
    label: '人工巡查',
    description: '由管理员巡查后手动打上的警示标签',
  },
} as const;

export const SITE_WARNING_TAG_SOURCE_KEYS = Object.keys(SITE_WARNING_TAG_SOURCES) as Array<
  keyof typeof SITE_WARNING_TAG_SOURCES
>;

export type SiteWarningTagSourceKey = (typeof SITE_WARNING_TAG_SOURCE_KEYS)[number];

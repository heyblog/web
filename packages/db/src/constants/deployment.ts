export const DEPLOYMENT_STATUSES = {
  PENDING: {
    label: '待执行',
    description: '部署任务已记录，等待实际执行',
  },
  RUNNING: {
    label: '执行中',
    description: '部署流程正在执行',
  },
  SELF_UPDATING: {
    label: '自更新中',
    description: '部署服务正在构建并替换自身二进制',
  },
  WAITING_RESUME: {
    label: '等待恢复',
    description: '部署服务已触发自更新，等待新进程恢复后续阶段',
  },
  RUNNING_INFRA_SYNC: {
    label: '同步配置中',
    description: '部署流程正在同步生产配置',
  },
  RUNNING_DB_MIGRATE: {
    label: '迁移数据库中',
    description: '部署流程正在执行数据库迁移',
  },
  RUNNING_SERVICES: {
    label: '更新服务中',
    description: '部署流程正在更新服务容器',
  },
  SUCCESS: {
    label: '成功',
    description: '部署流程执行成功',
  },
  FAILED: {
    label: '失败',
    description: '部署流程执行失败',
  },
  ROLLED_BACK: {
    label: '已回滚',
    description: '部署失败后已执行回滚',
  },
  SKIPPED: {
    label: '已跳过',
    description: '部署流程因无变更或策略原因被跳过',
  },
} as const;

export const DEPLOYMENT_STATUS_KEYS = Object.keys(DEPLOYMENT_STATUSES) as Array<
  keyof typeof DEPLOYMENT_STATUSES
>;

export type DeploymentStatusKey = (typeof DEPLOYMENT_STATUS_KEYS)[number];

export const DEPLOYMENT_MODULES = {
  WEB: {
    label: 'Web',
    description: '前端网站模块',
  },
  API: {
    label: 'API',
    description: 'Fastify API 服务模块',
  },
  WORKER: {
    label: 'Worker',
    description: '任务消费与抓取处理模块',
  },
  DEPLOYER: {
    label: 'Deployer',
    description: '部署服务模块',
  },
  STATUS: {
    label: 'Status',
    description: '状态页与监控模块',
  },
  CLOUDFLARE: {
    label: 'Cloudflare',
    description: 'Cloudflare Workers 与边缘能力模块',
  },
  ALL: {
    label: '全部模块',
    description: '覆盖全量模块的一次部署任务',
  },
  DB: {
    label: 'DB',
    description: '数据库模块，通常与公共模块全量更新同时记录',
  },
} as const;

export const DEPLOYMENT_MODULE_KEYS = Object.keys(DEPLOYMENT_MODULES) as Array<
  keyof typeof DEPLOYMENT_MODULES
>;

export type DeploymentModuleKey = (typeof DEPLOYMENT_MODULE_KEYS)[number];

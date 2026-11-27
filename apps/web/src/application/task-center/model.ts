import { TaskTransportError } from '../../api/task-center/task-center.browser.ts';
import type { Configuration, ScheduleInput } from '../../api/task-center/tasks.types.ts';
export function newSchedule(): ScheduleInput {
  return {
    name: '',
    kind: 'monitor',
    enabled: false,
    archived: false,
    revision: 0,
    configuration: {
      mode: 'interval',
      interval_seconds: 3600,
      cron: '0 * * * *',
      timezone: 'Asia/Shanghai',
      scope: 'all',
      site_ids: [],
      request_config_id: '',
      feed_mode: 'default',
      user_agent: 'HeyBlog/1.0',
      headers: {},
      timeout_seconds: 8,
      interval_ms: 1000,
      concurrency: 2,
      retries: 0,
      max_redirects: 3,
      run_timeout_seconds: 86400,
      signed: true,
      carrier: 'both',
      ttl_seconds: 300,
    },
  };
}
export function message(error: unknown): string {
  if (!(error instanceof Error)) throw error;
  if (error instanceof TaskTransportError) {
    if (error.status === 401) return '登录已过期，请重新登录。';
    if (error.status === 403) return '你没有执行此操作的权限。';
    if (error.status === 409)
      return error.currentState
        ? `操作未执行，运行当前为「${state(error.currentState)}」。`
        : '状态已变化，请刷新后重新操作。';
    if (error.status === 422) return '配置无效，请检查时间表达式、时区和请求参数。';
    if (error.status === 0) return '未能确认操作结果，请先刷新列表，不要重复提交。';
  }
  return '操作暂时无法完成，请稍后刷新重试。';
}
export function date(value: string | null): string {
  return value
    ? new Date(value).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false })
    : '—';
}
export function state(value: string): string {
  const labels: Readonly<Record<string, string>> = {
    pending: '等待执行',
    queued: '排队中',
    running: '执行中',
    pausing: '暂停中',
    paused: '已暂停',
    cancelling: '取消中',
    succeeded: '执行成功',
    partial_failed: '执行不完整',
    timed_out: '已超时',
    skipped: '已跳过',
    retry_wait: '等待重试',
    authorized: '已授权',
    sent: '已报告发送',
    received: '已收到结果',
    unknown: '发送情况未知',
    cancelled: '已取消',
    failed: '执行失败',
    generated: '待发布',
    published: '已发布',
    active: '使用中',
    retired: '已轮换',
    revoked: '已撤销',
  };
  return labels[value] ?? '状态待确认';
}
export function configurationRows(c: Configuration): readonly (readonly [string, string])[] {
  return [
    ['调度', c.mode === 'cron' ? c.cron : `每 ${c.interval_seconds} 秒`],
    ['时区', c.timezone],
    ['目标范围', c.scope === 'all' ? '全部可见站点' : c.site_ids.join('、')],
    ['User-Agent', c.user_agent],
    ['超时', `${c.timeout_seconds} 秒`],
    ['请求间隔', `${c.interval_ms} 毫秒`],
    ['并发', String(c.concurrency)],
    ['额外重试', String(c.retries)],
    ['最大重定向', String(c.max_redirects)],
    ['运行时限', `${c.run_timeout_seconds} 秒`],
    ['RSS 范围', c.feed_mode === 'default' ? '默认订阅源' : '全部启用订阅源'],
    ['请求签名', c.signed ? (c.carrier === 'both' ? 'UA＋Header' : '仅 Header') : '未签名'],
    [
      '请求头',
      Object.entries(c.headers)
        .map(([k, v]) => `${k}: ${v}`)
        .join('\n') || '无',
    ],
  ];
}
export const errorLabels: Readonly<Record<string, string>> = {
  target: '目标访问失败',
  platform: '平台调用异常',
  security: '目标安全检查未通过',
  signature: '签名不可用',
  backoff: '平台退避中',
  resolver: 'DNS 暂时不可用',
  cancelled: '已取消',
  invalid_feed: 'RSS 内容无效',
  target_http: '目标 HTTP 错误',
  incomplete_redirect: '未取得重定向后的完整结果',
};
export const kindLabels: Readonly<Record<string, string>> = {
  monitor: '站点检测',
  rss: 'RSS 抓取',
  verification: 'HTTP 认证',
};
export const sourceLabels: Readonly<Record<string, string>> = {
  scheduled: '定时',
  manual: '手动',
  event: '业务触发',
  retry: '失败重试',
  rerun: '全部重跑',
};
export const actionLabels: Readonly<Record<string, string>> = {
  pause: '暂停',
  resume: '恢复',
  cancel: '取消运行',
  archive: '归档',
  unarchive: '取消归档',
  cleanup: '清理详情',
  retry: '重试失败目标',
  rerun: '全部重新运行',
};
export function active(state: string): boolean {
  return ['queued', 'running', 'pausing', 'cancelling'].includes(state);
}
export function reason(value: string): string {
  const labels: Readonly<Record<string, string>> = {
    no_eligible_targets: '无符合条件目标',
    round_expired: '双路观察已超过五分钟',
    proof_mismatch: '验证证明不匹配',
    run_cancelled: '运行已取消',
    stale_target: '目标信息已变更',
    platform_backoff: '等待平台恢复',
    timeout: '达到运行时限',
    recovery_exhausted: '故障接管次数已用尽',
    interrupted: '等待故障恢复',
  };
  return labels[value] ?? errorLabels[value] ?? (value ? '执行未完成，请查看目标结果' : '');
}

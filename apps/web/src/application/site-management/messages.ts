export const claimMethodLabels = {
  DNS_TXT: 'DNS TXT',
  META: '页面 meta',
  FILE: '验证文件',
  MANUAL: '人工验证',
} as const;
export const claimStatusLabels = {
  PENDING: '待验证',
  VERIFIED: '已认证',
  REJECTED: '未通过',
  CANCELLED: '已取消',
} as const;
export const auditStatusLabels = {
  PENDING: '待审核',
  APPROVED: '已通过',
  REJECTED: '未通过',
} as const;
export const auditActionLabels = {
  CREATE: '新增站点',
  UPDATE: '资料变更',
  DELETE: '删除站点',
  RESTORE: '恢复站点',
} as const;
export class SiteManagementProblem extends Error {}
export async function managementError(response: Response): Promise<string> {
  if (response.status === 401) return '登录已过期，请重新登录。';
  if (response.status === 403) return '当前账号无权执行此操作。';
  if (response.status === 409) return '状态已变化，请刷新后重试。';
  if (response.status === 429) return '操作过于频繁，请稍后重试。';
  if (response.status >= 500) return '服务暂时不可用，请稍后重试。';
  return '操作未完成，请检查填写内容和验证配置后重试。';
}

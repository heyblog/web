import { buildSubmissionPayload, type EditableSubmission } from './site-submission.browser.ts';
import type {
  AuditAction,
  SiteAvailability,
  SiteSearchResult,
  SubmissionResult,
} from './site-submission.types';

const fallbackProblemMessage = '请求失败，请稍后重试。';

const problemMessages: Readonly<Record<string, string>> = {
  invalid_submission: '请检查当前步骤中标出的内容。',
  site_url_purpose_conflict:
    '主页、Feed、Sitemap 和友链页不能使用同一地址，请修改地址或清空可选项。',
  invalid_site_address: '请填写有效的 HTTP 或 HTTPS 主页地址。',
  site_address_conflict: '该站点已在目录中，请改为提交更新申请。',
  audit_not_found: '查询凭证无效或申请不存在。',
  site_not_found: '未找到该站点。',
  site_already_removed: '该站点已处于删除状态。',
  site_not_removed: '该站点当前无需恢复。',
  audit_already_reviewed: '该申请已处理，请刷新页面。',
  site_revision_changed: '站点数据已变化，请刷新后重新审核。',
  audit_conflicts_unresolved: '站点数据已变化，请刷新后重新审核。',
  taxonomy_permission_required: '当前账号无权新建分类或技术条目。',
  taxonomy_metadata_required: '请补全新程序或技术条目的资料。',
  invalid_tag: '所选标签已不可用，请刷新后重新选择。',
  invalid_component: '所选程序或技术已不可用，请刷新后重试。',
  invalid_program_dependency: '程序技术栈包含无效或循环依赖。',
  program_already_exists: '该程序已存在，请改为选择目录中的程序。',
  submission_no_changes: '未检测到可提交的修改。',
  submission_pending: '该站点已有待审核申请，请先查询处理进度。',
  review_comment_required: '驳回时请填写审核意见。',
  review_draft_changed: '修正稿已被其他审核者更新，请刷新后继续。',
  review_draft_forbidden: '当前申请不支持批准前修正。',
  forbidden: '你没有执行此操作的权限。',
  request_too_large: '提交内容过大。',
  unsupported_media_type: '提交格式不受支持。',
  bad_gateway: '服务暂时不可用，请稍后重试。',
  internal_error: '服务暂时不可用，请稍后重试。',
};

export interface BrowserRequestOptions {
  readonly signal?: AbortSignal;
  readonly fetch?: typeof globalThis.fetch;
}

export class SiteSubmissionProblem extends Error {
  readonly code: string;

  constructor(code: string, message: string) {
    super(message);
    this.code = code;
    this.name = 'SiteSubmissionProblem';
  }
}

interface ProblemPayload {
  readonly code?: unknown;
}

async function responseProblem(response: Response): Promise<SiteSubmissionProblem> {
  const payload = (await response.json().catch(() => null)) as ProblemPayload | null;
  const code = typeof payload?.code === 'string' ? payload.code : 'request_failed';
  return new SiteSubmissionProblem(code, problemMessages[code] ?? fallbackProblemMessage);
}

export async function problemDetail(response: Response): Promise<string> {
  return (await responseProblem(response)).message;
}

export function submissionEndpoint(action: AuditAction, siteShortID: string): string {
  if (action === 'CREATE') return '/api/site-submissions/create';
  const suffix = action === 'UPDATE' ? 'update' : action === 'DELETE' ? 'delete' : 'restore';
  return '/api/site-submissions/' + encodeURIComponent(siteShortID) + '/' + suffix;
}

export async function submitForm(
  action: AuditAction,
  form: EditableSubmission,
  options: BrowserRequestOptions = {},
): Promise<SubmissionResult> {
  const request = options.fetch ?? globalThis.fetch;
  const response = await request(submissionEndpoint(action, form.siteShortId), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(buildSubmissionPayload(form, action)),
    signal: options.signal,
  });
  if (!response.ok) throw await responseProblem(response);
  return (await response.json()) as SubmissionResult;
}

export async function checkSiteAvailability(
  url: string,
  options: BrowserRequestOptions = {},
): Promise<SiteAvailability> {
  const request = options.fetch ?? globalThis.fetch;
  const endpoint = '/api/site-submissions/availability?url=' + encodeURIComponent(url);
  const response = await request(endpoint, { signal: options.signal });
  if (!response.ok) throw await responseProblem(response);
  return (await response.json()) as SiteAvailability;
}

export function siteAvailabilityTarget(site: SiteSearchResult): string {
  const action = site.visibility === 'REMOVED' ? 'restore' : 'update';
  return '/site/submissions/' + action + '?short_id=' + encodeURIComponent(site.short_id);
}

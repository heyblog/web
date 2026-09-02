import type { EditableSubmission } from './site-submission.browser';
import type { AuditAction } from './site-submission.types';

export interface StepValidation {
  readonly valid: boolean;
  readonly message: string;
}

const siteShortIDPattern = /^[0-9A-Za-z]{9}$/;

export function isSiteShortID(value: string): boolean {
  return siteShortIDPattern.test(value);
}

function isHTTPURL(value: string): boolean {
  try {
    const parsed = new URL(value);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
  } catch {
    return false;
  }
}

export function validateSubmissionStep(
  action: AuditAction,
  form: EditableSubmission,
  step: number,
): StepValidation {
  if (step === 0) {
    if (action !== 'CREATE' && !isSiteShortID(form.siteShortId))
      return { valid: false, message: '请先选择目标站点。' };
    if (action === 'DELETE' || action === 'RESTORE') return { valid: true, message: '' };
    if (!form.name.trim()) return { valid: false, message: '请填写站点名称。' };
    if (!isHTTPURL(form.url.trim()))
      return { valid: false, message: '请填写有效的 HTTP 或 HTTPS 主页地址。' };
  }
  if (step === 1 && (action === 'CREATE' || action === 'UPDATE')) {
    const feeds = form.feeds.filter((feed) => feed.url.trim());
    if (
      feeds.some((feed) => !feed.name.trim() || !isHTTPURL(new URL(feed.url, form.url).toString()))
    )
      return { valid: false, message: '请补全每个 Feed 的名称和有效地址。' };
    if (feeds.length > 0 && feeds.filter((feed) => feed.isDefault).length !== 1)
      return { valid: false, message: '请选择且仅选择一个默认 Feed。' };
    if (
      new Set(feeds.map((feed) => new URL(feed.url, form.url).toString().toLowerCase())).size !==
      feeds.length
    )
      return { valid: false, message: 'Feed 地址不能重复。' };
  }
  if (step === 2 && (action === 'CREATE' || action === 'UPDATE')) {
    if (form.tags.length === 0 || form.tags.filter((tag) => tag.role === 'PRIMARY').length !== 1)
      return { valid: false, message: '请选择至少一个标签，并指定一个主标签。' };
    if (form.program.kind === 'none') return { valid: false, message: '请选择站点程序。' };
    if (form.program.kind === 'custom') {
      if (!form.program.name.trim() || form.program.name.trim().length > 128)
        return { valid: false, message: '请填写不超过 128 个字符的程序名称。' };
      if (form.program.isOpenSource === null)
        return { valid: false, message: '请选择程序是否开源。' };
      if (
        !isHTTPURL(form.program.homepageURL.trim()) &&
        !isHTTPURL(form.program.repositoryURL.trim())
      )
        return { valid: false, message: '请填写程序官网或代码仓库。' };
      if (
        form.program.dependencies.some(
          (dependency) =>
            !dependency.id &&
            (dependency.isOpenSource === null ||
              (!isHTTPURL(dependency.homepageURL?.trim() ?? '') &&
                !isHTTPURL(dependency.repositoryURL?.trim() ?? ''))),
        )
      )
        return { valid: false, message: '请补全自定义技术的链接和开源状态。' };
    }
  }
  const isFinalStep = step === submissionStepCount(action) - 1;
  if (isFinalStep && action !== 'CREATE' && !form.reason.trim())
    return { valid: false, message: '请填写申请原因。' };
  if (isFinalStep && form.notifyByEmail && !form.contactEmail.trim())
    return { valid: false, message: '接收邮件通知需要填写邮箱。' };
  return { valid: true, message: '' };
}

export function submissionStepCount(action: AuditAction): number {
  return action === 'CREATE' || action === 'UPDATE' ? 4 : 2;
}

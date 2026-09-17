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

function resolveHTTPURL(value: string, base: string): string | null {
  try {
    const parsed = new URL(value, base);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? parsed.toString() : null;
  } catch {
    return null;
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
    const feedURLs = feeds.map((feed) => resolveHTTPURL(feed.url, form.url));
    if (feeds.some((feed, index) => !feed.name.trim() || feedURLs[index] === null))
      return { valid: false, message: '请补全每个 Feed 的名称和有效地址。' };
    if (feeds.length > 0 && feeds.filter((feed) => feed.isDefault).length !== 1)
      return { valid: false, message: '请选择且仅选择一个默认 Feed。' };
    if (new Set(feedURLs.map((url) => url?.toLocaleLowerCase())).size !== feeds.length)
      return { valid: false, message: 'Feed 地址不能重复。' };
  }
  if (step === 2 && (action === 'CREATE' || action === 'UPDATE')) {
    const level1Tags = form.tags.filter((tag) => tag.level === 1 && tag.role === 'PRIMARY');
    const level2Tags = form.tags.filter((tag) => tag.level === 2 && tag.role === 'SECONDARY');
    if (level1Tags.length !== 1 || level2Tags.length !== 1)
      return { valid: false, message: '请选择完整的一级和二级分类。' };
    if (level2Tags[0]?.parent_id !== level1Tags[0]?.id)
      return { valid: false, message: '所选二级分类不属于当前一级分类。' };
    if (form.tags.filter((tag) => tag.role === 'TERTIARY').length > 20)
      return { valid: false, message: '三级标签最多选择 20 个。' };
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
  const hasContactName = form.contactName.trim().length > 0;
  const hasContactEmail = form.contactEmail.trim().length > 0;
  if (isFinalStep && hasContactName !== hasContactEmail)
    return { valid: false, message: '称呼和邮箱需要同时填写，或同时留空。' };
  if (isFinalStep && form.notifyByEmail && !form.contactEmail.trim())
    return { valid: false, message: '接收邮件通知需要填写称呼和邮箱。' };
  return { valid: true, message: '' };
}

export function submissionStepCount(action: AuditAction): number {
  return action === 'CREATE' || action === 'UPDATE' ? 4 : 2;
}

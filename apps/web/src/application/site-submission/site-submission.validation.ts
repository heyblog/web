import type { EditableSubmission } from './site-submission.browser';
import type { AuditAction } from './site-submission.types';

export interface StepValidation {
  readonly valid: boolean;
  readonly message: string;
}

export interface AuxiliaryURLValidation {
  readonly feedMessages: Readonly<Record<string, string>>;
  readonly sitemapMessage: string;
  readonly linkPageMessage: string;
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

function cleanRootRelativePath(value: string): string {
  const segments: string[] = [];
  for (const segment of value.split('/')) {
    if (!segment || segment === '.') continue;
    if (segment === '..') {
      segments.pop();
      continue;
    }
    segments.push(segment);
  }
  return `/${segments.join('/')}`;
}

function pathWithinBase(candidate: string, base: string): boolean {
  return base === '/' || candidate === base || candidate.startsWith(`${base}/`);
}

function normalizedHostname(url: URL): string {
  return url.hostname.endsWith('.') ? url.hostname.slice(0, -1) : url.hostname;
}

function normalizedLocationKey(value: string, siteURL: string): string | null {
  try {
    const site = new URL(siteURL.trim());
    if (site.protocol !== 'http:' && site.protocol !== 'https:') return null;
    const basePath = cleanRootRelativePath(site.pathname);
    const trimmed = value.trim();
    if (!trimmed || trimmed.startsWith('//')) return null;

    let parsed: URL;
    try {
      parsed = new URL(trimmed);
    } catch {
      const base = new URL(site.origin);
      base.pathname = `${basePath === '/' ? '' : basePath}/`;
      parsed = new URL(trimmed, base);
      const resolvedPath = cleanRootRelativePath(parsed.pathname);
      if (!trimmed.startsWith('/') && !pathWithinBase(resolvedPath, basePath)) return null;
    }
    if (
      (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') ||
      parsed.username ||
      parsed.password
    )
      return null;

    parsed.hash = '';
    const pathname = cleanRootRelativePath(parsed.pathname);
    if (normalizedHostname(parsed) === normalizedHostname(site) && parsed.port === '')
      return pathname + parsed.search;
    parsed.hostname = normalizedHostname(parsed);
    parsed.pathname = pathname;
    return parsed.toString();
  } catch {
    return null;
  }
}

export function validateAuxiliaryURLs(form: EditableSubmission): AuxiliaryURLValidation {
  const homepageKey = normalizedLocationKey(form.url, form.url);
  const feedMessages: Record<string, string> = {};
  const feedIDsByKey = new Map<string, string>();

  for (const feed of form.feeds) {
    if (!feed.url.trim()) continue;
    const key = normalizedLocationKey(feed.url, form.url);
    if (key === null) {
      feedMessages[feed.id] = '请填写有效的 HTTP、HTTPS 或站内相对 Feed 地址。';
      continue;
    }
    if (key === homepageKey) {
      feedMessages[feed.id] = 'Feed 地址不能与主页地址相同。';
      continue;
    }
    const duplicateID = feedIDsByKey.get(key);
    if (duplicateID) {
      feedMessages[duplicateID] = 'Feed 地址不能重复。';
      feedMessages[feed.id] = 'Feed 地址不能重复。';
      continue;
    }
    feedIDsByKey.set(key, feed.id);
  }

  const resourceMessage = (value: string, label: 'Sitemap' | '友链页'): string => {
    if (!value.trim()) return '';
    const fieldName = label === 'Sitemap' ? 'Sitemap 地址' : '友链页地址';
    const key = normalizedLocationKey(value, form.url);
    if (key === null) return `请填写有效的 HTTP、HTTPS 或站内相对${fieldName}。`;
    if (key === homepageKey) return `${fieldName}不能与主页地址相同。`;
    return '';
  };
  let sitemapMessage = resourceMessage(form.sitemap, 'Sitemap');
  let linkPageMessage = resourceMessage(form.linkPage, '友链页');
  const sitemapKey = normalizedLocationKey(form.sitemap, form.url);
  const linkPageKey = normalizedLocationKey(form.linkPage, form.url);
  if (
    !sitemapMessage &&
    !linkPageMessage &&
    form.sitemap.trim() &&
    form.linkPage.trim() &&
    sitemapKey === linkPageKey
  ) {
    sitemapMessage = 'Sitemap 和友链页不能使用同一地址。';
    linkPageMessage = sitemapMessage;
  }

  return { feedMessages, sitemapMessage, linkPageMessage };
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
    if (feeds.some((feed) => !feed.name.trim()))
      return { valid: false, message: '请补全每个 Feed 的名称和有效地址。' };
    const auxiliaryURLs = validateAuxiliaryURLs(form);
    for (const feed of feeds) {
      const message = auxiliaryURLs.feedMessages[feed.id];
      if (message) return { valid: false, message };
    }
    if (auxiliaryURLs.sitemapMessage)
      return { valid: false, message: auxiliaryURLs.sitemapMessage };
    if (auxiliaryURLs.linkPageMessage)
      return { valid: false, message: auxiliaryURLs.linkPageMessage };
    if (feeds.length > 0 && feeds.filter((feed) => feed.isDefault).length !== 1)
      return { valid: false, message: '请选择且仅选择一个默认 Feed。' };
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

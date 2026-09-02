import type {
  AuditAction,
  ComponentInput,
  DependencyRole,
  FeedFormat,
  Option,
  PublicSnapshot,
  SiteInput,
  SubmissionOptions,
  SubmissionPayload,
  SubmissionResult,
} from './site-submission.types';

const problemMessages: Readonly<Record<string, string>> = {
  invalid_submission: '请检查当前步骤中标出的内容。',
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

export interface FeedDraft {
  id: string;
  name: string;
  url: string;
  format: FeedFormat;
  isDefault: boolean;
}
export interface SelectedTag {
  id: string;
  name: string;
  role: 'PRIMARY' | 'SECONDARY';
}
export interface DependencyDraft {
  id: string;
  name: string;
  role: DependencyRole;
  isOpenSource?: boolean | null;
  homepageURL?: string;
  repositoryURL?: string;
}
export type ProgramDraft =
  | { kind: 'none' }
  | { kind: 'other'; id: string; name: string }
  | { kind: 'existing'; id: string; name: string; dependencies: DependencyDraft[] }
  | {
      kind: 'custom';
      name: string;
      isOpenSource: boolean | null;
      homepageURL: string;
      repositoryURL: string;
      dependencies: DependencyDraft[];
    };
export interface EditableSubmission {
  siteShortId: string;
  name: string;
  url: string;
  summary: string;
  feeds: FeedDraft[];
  sitemap: string;
  linkPage: string;
  tags: SelectedTag[];
  program: ProgramDraft;
  reason: string;
  contactName: string;
  contactEmail: string;
  notifyByEmail: boolean;
}

let draftSequence = 0;
function draftID(prefix: string): string {
  draftSequence += 1;
  return `${prefix}-${draftSequence}`;
}

export function emptySubmission(): EditableSubmission {
  return {
    siteShortId: '',
    name: '',
    url: 'https://',
    summary: '',
    feeds: [],
    sitemap: '',
    linkPage: '',
    tags: [],
    program: { kind: 'none' },
    reason: '',
    contactName: '',
    contactEmail: '',
    notifyByEmail: false,
  };
}

export function addFeed(form: EditableSubmission): void {
  if (form.feeds.length >= 8) return;
  form.feeds.push({
    id: draftID('feed'),
    name: form.feeds.length === 0 ? '默认订阅' : '',
    url: '',
    format: 'UNKNOWN',
    isDefault: form.feeds.length === 0,
  });
}
export function setDefaultFeed(form: EditableSubmission, id: string): void {
  for (const feed of form.feeds) feed.isDefault = feed.id === id;
}
export function removeFeed(form: EditableSubmission, id: string): void {
  const removedDefault = form.feeds.find((feed) => feed.id === id)?.isDefault === true;
  form.feeds = form.feeds.filter((feed) => feed.id !== id);
  if (removedDefault && form.feeds[0]) setDefaultFeed(form, form.feeds[0].id);
}
export function selectTag(form: EditableSubmission, option: Option): void {
  if (form.tags.length >= 12 || form.tags.some((tag) => tag.id === option.id)) return;
  form.tags.push({
    id: option.id,
    name: option.name,
    role: form.tags.length === 0 ? 'PRIMARY' : 'SECONDARY',
  });
}
export function makePrimaryTag(form: EditableSubmission, id: string): void {
  for (const tag of form.tags) tag.role = tag.id === id ? 'PRIMARY' : 'SECONDARY';
}
export function removeTag(form: EditableSubmission, id: string): void {
  const removedPrimary = form.tags.find((tag) => tag.id === id)?.role === 'PRIMARY';
  form.tags = form.tags.filter((tag) => tag.id !== id);
  if (removedPrimary && form.tags[0]) makePrimaryTag(form, form.tags[0].id);
}

export function syncURLSuggestions(
  form: EditableSubmission,
  previousURL: string,
  nextURL: string,
): void {
  const shouldUpdate = (value: string): boolean =>
    !value.trim() || value.trim() === previousURL.trim();
  const firstFeed = form.feeds[0];
  if (firstFeed && shouldUpdate(firstFeed.url)) firstFeed.url = nextURL.trim();
  if (shouldUpdate(form.sitemap)) form.sitemap = nextURL.trim();
  if (shouldUpdate(form.linkPage)) form.linkPage = nextURL.trim();
}

function component(
  id: string,
  name: string,
  role: ComponentInput['role'],
  isOpenSource: boolean | null = null,
  homepageURL = '',
  repositoryURL = '',
): ComponentInput {
  return {
    id,
    suggested_name: name,
    role,
    homepage_url: homepageURL,
    repository_url: repositoryURL,
    is_open_source: isOpenSource,
  };
}
function architecture(program: ProgramDraft): {
  components: ComponentInput[];
  dependencies: ComponentInput[];
} {
  switch (program.kind) {
    case 'none':
      return { components: [], dependencies: [] };
    case 'other':
      return { components: [component(program.id, '', 'SITE_PROGRAM')], dependencies: [] };
    case 'existing':
      return { components: [component(program.id, '', 'SITE_PROGRAM')], dependencies: [] };
    case 'custom': {
      const repositoryURL = program.repositoryURL.trim();
      const homepageURL = program.homepageURL.trim() || repositoryURL;
      return {
        components: [
          component(
            '',
            program.name.trim(),
            'SITE_PROGRAM',
            program.isOpenSource,
            homepageURL,
            repositoryURL,
          ),
        ],
        dependencies: program.dependencies.map((dependency) =>
          component(
            dependency.id,
            dependency.id ? '' : dependency.name.trim(),
            dependency.role,
            dependency.isOpenSource ?? null,
            dependency.homepageURL ?? '',
            dependency.repositoryURL ?? '',
          ),
        ),
      };
    }
  }
}
export function buildSubmissionPayload(
  form: EditableSubmission,
  action: AuditAction = 'UPDATE',
): SubmissionPayload {
  const { components, dependencies } = architecture(form.program);
  const site: SiteInput = {
    name: form.name.trim(),
    url: form.url.trim(),
    summary: form.summary.trim(),
    feeds: form.feeds
      .filter((feed) => feed.url.trim())
      .map((feed) => ({
        name: feed.name.trim(),
        url: feed.url.trim(),
        format: feed.format,
        is_default: feed.isDefault,
      })),
    resources: [
      ...(form.sitemap.trim() ? [{ kind: 'SITEMAP' as const, url: form.sitemap.trim() }] : []),
      ...(form.linkPage.trim() ? [{ kind: 'LINK_PAGE' as const, url: form.linkPage.trim() }] : []),
    ],
    tags: form.tags.map((tag) => ({
      id: tag.id,
      suggested_name: '',
      slug: '',
      description: '',
      role: tag.role,
    })),
    components,
    program_dependencies: dependencies,
  };
  return {
    site,
    ...(action === 'CREATE' ? {} : { reason: form.reason.trim() }),
    contact: {
      name: form.contactName.trim(),
      email: form.contactEmail.trim(),
      notify_by_email: form.notifyByEmail,
    },
  };
}

export function applySnapshot(
  form: EditableSubmission,
  snapshot: PublicSnapshot,
  options: SubmissionOptions,
): void {
  form.siteShortId = snapshot.short_id ?? form.siteShortId;
  form.name = snapshot.name;
  form.url = `${snapshot.scheme}://${snapshot.normalized_host}${snapshot.base_path}`;
  form.summary = snapshot.summary;
  form.feeds = snapshot.feeds.map((feed) => ({
    id: draftID('feed'),
    name: feed.name,
    url: feed.url,
    format: feed.format,
    isDefault: feed.is_default,
  }));
  form.sitemap = snapshot.resources.find((item) => item.kind === 'SITEMAP')?.url ?? '';
  form.linkPage = snapshot.resources.find((item) => item.kind === 'LINK_PAGE')?.url ?? '';
  form.tags = snapshot.tags.map((tag) => ({
    id: tag.id,
    name: tag.name || options.tags.find((option) => option.id === tag.id)?.name || tag.id,
    role: tag.role,
  }));
  const program = snapshot.components.find((item) => item.role === 'SITE_PROGRAM');
  if (!program) {
    form.program = { kind: 'none' };
    return;
  }
  if (program.id === options.private_program_id) {
    form.program = { kind: 'other', id: program.id, name: program.name };
    return;
  }
  const dependencies = snapshot.program_dependencies
    .filter((item) => item.role === 'FRAMEWORK' || item.role === 'LANGUAGE')
    .map((item) => ({
      id: item.id,
      name:
        item.name ||
        item.suggested_name ||
        options.components.find((candidate) => candidate.id === item.id)?.name ||
        item.id,
      role: item.role === 'FRAMEWORK' ? ('FRAMEWORK' as const) : ('LANGUAGE' as const),
      isOpenSource: item.is_open_source,
      homepageURL: item.homepage_url,
      repositoryURL: item.repository_url,
    }));
  if (!program.id) {
    form.program = {
      kind: 'custom',
      name: program.suggested_name || program.name,
      isOpenSource: program.is_open_source,
      homepageURL: program.homepage_url,
      repositoryURL: program.repository_url,
      dependencies,
    };
    return;
  }
  form.program = {
    kind: 'existing',
    id: program.id,
    name:
      program.name || options.components.find((item) => item.id === program.id)?.name || program.id,
    dependencies,
  };
}

export function submissionEndpoint(action: AuditAction, siteShortId: string): string {
  if (action === 'CREATE') return '/api/site-submissions/create';
  const suffix = action === 'UPDATE' ? 'update' : action === 'DELETE' ? 'delete' : 'restore';
  return `/api/site-submissions/${encodeURIComponent(siteShortId)}/${suffix}`;
}
export async function submitForm(
  action: AuditAction,
  form: EditableSubmission,
): Promise<SubmissionResult> {
  const response = await fetch(submissionEndpoint(action, form.siteShortId), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(buildSubmissionPayload(form, action)),
  });
  if (!response.ok) throw new Error(await problemDetail(response));
  return (await response.json()) as SubmissionResult;
}
export async function problemDetail(response: Response): Promise<string> {
  const payload: unknown = await response.json().catch(() => null);
  if (
    typeof payload === 'object' &&
    payload !== null &&
    'code' in payload &&
    typeof payload.code === 'string'
  )
    return problemMessages[payload.code] ?? '请求失败，请稍后重试。';
  return '请求失败，请稍后重试。';
}

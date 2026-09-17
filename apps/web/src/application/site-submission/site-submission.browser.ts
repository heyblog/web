import { nextDraftID } from './site-submission.draft-id.browser.ts';
import type {
  AuditAction,
  ComponentInput,
  DependencyRole,
  FeedFormat,
  SiteInput,
  SubmissionPayload,
} from './site-submission.types';

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
  role: 'PRIMARY' | 'SECONDARY' | 'TERTIARY';
  level?: 1 | 2 | 3;
  parent_id?: string | null;
  suggestedName?: string;
  slug?: string;
  description?: string;
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
    id: nextDraftID('feed'),
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
      id: tag.suggestedName ? '' : tag.id,
      suggested_name: tag.suggestedName ?? '',
      slug: tag.slug?.trim() ?? '',
      description: tag.description?.trim() ?? '',
      role: tag.role,
      ...(tag.level === undefined ? {} : { level: tag.level }),
      ...(tag.parent_id === undefined ? {} : { parent_id: tag.parent_id }),
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

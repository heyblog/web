import type {
  PublicSnapshot,
  SubmissionOptions,
  TagInput,
  TagSnapshot,
} from '../../api/site-submission/site-submission.types.ts';

import type { EditableSubmission } from './site-submission.browser.ts';
import { nextDraftID } from './site-submission.draft-id.browser.ts';

export function applySnapshot(
  form: EditableSubmission,
  snapshot: PublicSnapshot,
  options: SubmissionOptions,
): void {
  form.siteShortId = snapshot.short_id ?? form.siteShortId;
  form.name = snapshot.name;
  form.url = `${snapshot.scheme}://${snapshot.normalized_host}${snapshot.base_path}`;
  form.summary = snapshot.summary;
  form.accessScope =
    snapshot.access_scope === 'CN_ONLY' || snapshot.access_scope === 'GLOBAL_ONLY'
      ? snapshot.access_scope
      : 'ALL';
  form.feeds = snapshot.feeds.map((feed) => ({
    id: nextDraftID('feed'),
    name: feed.name,
    url: feed.url,
    format: feed.format,
    isDefault: feed.is_default,
  }));
  form.sitemap = snapshot.resources.find((item) => item.kind === 'SITEMAP')?.url ?? '';
  form.linkPage = snapshot.resources.find((item) => item.kind === 'LINK_PAGE')?.url ?? '';
  form.tags = snapshot.tags.filter(isEditableTagSnapshot).map((tag) => {
    const labelID =
      tag.label_id ||
      options.tags.find((option) => option.id === tag.id && option.name === tag.name)?.label_id ||
      options.tags.find((option) => option.id === tag.id)?.label_id;
    return {
      id: tag.id || nextDraftID('tag'),
      ...(labelID ? { label_id: labelID } : {}),
      name:
        tag.name ||
        tag.suggested_name ||
        options.tags.find(
          (option) =>
            option.id === tag.id &&
            (!tag.label_id || option.label_id === tag.label_id) &&
            (option.role === tag.role || option.level === tag.level),
        )?.name ||
        tag.id,
      role: tag.role,
      level: tag.level,
      parent_id: tag.parent_id,
      suggestedName: tag.suggested_name || undefined,
      slug: tag.slug,
      description: tag.description,
    };
  });
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

function isEditableTagSnapshot(
  tag: TagSnapshot,
): tag is TagSnapshot & { readonly role: TagInput['role'] } {
  return tag.role !== 'WARNING';
}

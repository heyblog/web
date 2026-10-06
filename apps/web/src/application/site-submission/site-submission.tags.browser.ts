import type { Option } from '../../api/site-submission/site-submission.types.ts';

import type { EditableSubmission, SelectedTag } from './site-submission.browser.ts';
import { matchesSubmissionOption } from './site-submission.search.ts';

export function tertiaryTagOptions(
  options: readonly Option[],
  _tags: readonly SelectedTag[],
): Option[] {
  const candidates = new Map<string, Option>();
  const seenNames = new Set<string>();
  for (const option of options) {
    const name = option.name.trim().toLocaleLowerCase();
    const key = tagOptionKey(option);
    if (option.role === 'WARNING' || candidates.has(key) || seenNames.has(name)) continue;
    // Labels remain visible so the picker can explain why an equivalent name is unavailable.
    candidates.set(key, option);
    seenNames.add(name);
  }
  return [...candidates.values()];
}

export function tagOptionKey(option: Pick<Option, 'id' | 'label_id'>): string {
  return option.label_id || option.id;
}

export function tagSelectionReason(option: Option, tags: readonly SelectedTag[]): string {
  const existing = tags.find(
    (tag) =>
      tag.id === option.id ||
      tag.name.trim().toLocaleLowerCase() === option.name.trim().toLocaleLowerCase(),
  );
  return existing ? `已选择“${existing.name}”` : '';
}

export function matchingTagOptions(query: string, options: readonly Option[]): Option[] {
  const term = query.trim().toLocaleLowerCase();
  const concepts = new Set(
    options
      .filter((option) =>
        [option.name, ...(option.synonyms ?? [])].some((name) =>
          matchesSubmissionOption(term, name),
        ),
      )
      .map((option) => option.id),
  );
  return options.filter((option) => concepts.has(option.id));
}

export function selectClassificationTag(form: EditableSubmission, option: Option): void {
  if (option.level !== 1 && option.level !== 2) return;
  const level = option.level;
  form.tags = form.tags.filter(
    (tag) =>
      !(
        tag.role === 'TERTIARY' &&
        (tag.id === option.id ||
          tag.name.trim().toLocaleLowerCase() === option.name.trim().toLocaleLowerCase())
      ) &&
      tag.role !== (level === 1 ? 'PRIMARY' : 'SECONDARY') &&
      (level !== 1 || tag.role !== 'SECONDARY'),
  );
  const selected: SelectedTag = {
    id: option.id,
    ...(option.label_id ? { label_id: option.label_id } : {}),
    name: option.name,
    role: level === 1 ? 'PRIMARY' : 'SECONDARY',
    level,
  };
  if (option.parent_id !== undefined) selected.parent_id = option.parent_id;
  form.tags.push(selected);
}

export function selectTertiaryTag(form: EditableSubmission, option: Option): void {
  const normalizedName = option.name.trim().toLocaleLowerCase();
  if (
    form.tags.some(
      (tag) =>
        (option.id !== '' && tag.id === option.id) ||
        tag.name.trim().toLocaleLowerCase() === normalizedName,
    ) ||
    form.tags.filter((tag) => tag.role === 'TERTIARY').length >= 20
  )
    return;
  const selected: SelectedTag = {
    id: option.id,
    ...(option.label_id ? { label_id: option.label_id } : {}),
    name: option.name.trim(),
    role: 'TERTIARY',
    level: 3,
  };
  if (option.is_custom) {
    selected.suggestedName = option.name.trim();
    selected.slug = '';
    selected.description = '';
  }
  form.tags.push(selected);
}

export function removeTag(form: EditableSubmission, id: string, role?: SelectedTag['role']): void {
  const removedTag = form.tags.find(
    (tag) => tag.id === id && (role === undefined || tag.role === role),
  );
  form.tags = form.tags.filter((tag) => tag.id !== id || (role !== undefined && tag.role !== role));
  if (removedTag?.role === 'PRIMARY')
    form.tags = form.tags.filter((tag) => tag.role !== 'SECONDARY');
}

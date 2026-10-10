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
    const name = option.name.trim().toLowerCase();
    const key = tagOptionKey(option);
    if (option.role === 'WARNING' || candidates.has(key) || seenNames.has(name)) continue;
    // Keep selected tags visible so the picker can explain why they are unavailable.
    candidates.set(key, option);
    seenNames.add(name);
  }
  return [...candidates.values()];
}

export function tagOptionKey(option: Pick<Option, 'id'>): string {
  return option.id;
}

export function tagSelectionReason(option: Option, tags: readonly SelectedTag[]): string {
  const existing = tags.find(
    (tag) =>
      tag.id === option.id || tag.name.trim().toLowerCase() === option.name.trim().toLowerCase(),
  );
  return existing ? `已选择“${existing.name}”` : '';
}

export function matchingTagOptions(query: string, options: readonly Option[]): Option[] {
  const term = query.trim().toLowerCase();
  return options.filter((option) => matchesSubmissionOption(term, option.name));
}

export function selectClassificationTag(form: EditableSubmission, option: Option): void {
  if (option.level !== 1 && option.level !== 2) return;
  const level = option.level;
  form.tags = form.tags.filter(
    (tag) =>
      !(
        tag.role === 'TERTIARY' &&
        (tag.id === option.id || tag.name.trim().toLowerCase() === option.name.trim().toLowerCase())
      ) &&
      tag.role !== (level === 1 ? 'PRIMARY' : 'SECONDARY') &&
      (level !== 1 || tag.role !== 'SECONDARY'),
  );
  const selected: SelectedTag = {
    id: option.id,
    name: option.name,
    role: level === 1 ? 'PRIMARY' : 'SECONDARY',
    level,
  };
  if (option.parent_id !== undefined) selected.parent_id = option.parent_id;
  form.tags.push(selected);
}

export function selectTertiaryTag(form: EditableSubmission, option: Option): void {
  const normalizedName = option.name.trim().toLowerCase();
  if (
    form.tags.some(
      (tag) =>
        (option.id !== '' && tag.id === option.id) ||
        tag.name.trim().toLowerCase() === normalizedName,
    ) ||
    form.tags.filter((tag) => tag.role === 'TERTIARY').length >= 20
  )
    return;
  const selected: SelectedTag = {
    id: option.id,
    name: option.name.trim(),
    role: 'TERTIARY',
    level: 3,
  };
  if (option.is_custom) {
    selected.suggestedName = option.name.trim();
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

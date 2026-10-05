import type { Option } from '../../api/site-submission/site-submission.types.ts';

import type { EditableSubmission, SelectedTag } from './site-submission.browser.ts';

export function tertiaryTagOptions(
  options: readonly Option[],
  tags: readonly SelectedTag[],
): Option[] {
  const candidates = new Map<string, Option>();
  const seenIDs = new Set<string>();
  for (const option of options) {
    const name = option.name.trim().toLocaleLowerCase();
    if (
      option.role === 'WARNING' ||
      seenIDs.has(option.id) ||
      candidates.has(name) ||
      tags.some(
        (tag) =>
          tag.id === option.id ||
          tag.name.trim().toLocaleLowerCase() === option.name.trim().toLocaleLowerCase(),
      )
    )
      continue;
    candidates.set(name, option);
    seenIDs.add(option.id);
  }
  return [...candidates.values()];
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

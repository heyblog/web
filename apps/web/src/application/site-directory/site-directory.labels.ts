import type { SiteDirectoryOption } from '../../api/sites/site-directory.types.ts';

export function selectedDirectoryOption(
  options: readonly SiteDirectoryOption[],
  slug: string,
  labelID?: string,
): SiteDirectoryOption | undefined {
  return (
    options.find((option) => option.value === slug && option.label_id === labelID) ??
    options.find((option) => option.value === slug)
  );
}

export function directoryOptionSelected(
  options: readonly SiteDirectoryOption[],
  option: SiteDirectoryOption,
  slugs: readonly string[],
  labelIDs: readonly string[] = [],
): boolean {
  if (!slugs.includes(option.value)) return false;
  const chosenID = labelIDs.find((id) =>
    options.some((candidate) => candidate.value === option.value && candidate.label_id === id),
  );
  return selectedDirectoryOption(options, option.value, chosenID) === option;
}

export function matchingDirectoryOptions(
  options: readonly SiteDirectoryOption[],
  query: string,
): readonly SiteDirectoryOption[] {
  const term = query.trim().toLocaleLowerCase();
  const matching = new Set(
    options
      .filter((option) =>
        [option.label, ...(option.synonyms ?? [])].some((label) =>
          label.toLocaleLowerCase().includes(term),
        ),
      )
      .map((option) => option.value),
  );
  return options.filter((option) => matching.has(option.value));
}

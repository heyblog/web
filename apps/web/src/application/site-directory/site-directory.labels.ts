import type { SiteDirectoryOption } from '../../api/sites/site-directory.types.ts';

export function selectedDirectoryOption(
  options: readonly SiteDirectoryOption[],
  name: string,
): SiteDirectoryOption | undefined {
  return options.find((option) => option.value.trim().toLowerCase() === name.trim().toLowerCase());
}

export function directoryOptionSelected(
  options: readonly SiteDirectoryOption[],
  option: SiteDirectoryOption,
  names: readonly string[],
): boolean {
  return names.some((name) => selectedDirectoryOption(options, name) === option);
}

export function matchingDirectoryOptions(
  options: readonly SiteDirectoryOption[],
  query: string,
): readonly SiteDirectoryOption[] {
  const term = query.trim().toLowerCase();
  return options.filter((option) => option.label.toLowerCase().includes(term));
}

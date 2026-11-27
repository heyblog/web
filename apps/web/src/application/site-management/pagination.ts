export const listPageLimit = 1_000_000;

export function listPageNumber(search: URLSearchParams): number {
  return Math.min(listPageLimit, Math.max(1, Number.parseInt(search.get('page') ?? '1', 10) || 1));
}

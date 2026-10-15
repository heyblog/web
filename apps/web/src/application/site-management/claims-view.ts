export const claimViews = ['create', 'records'] as const;
export type ClaimView = (typeof claimViews)[number];

export function claimWorkspaceView(search: URLSearchParams): ClaimView {
  const explicit = claimViews.find((view) => view === search.get('view'));
  if (explicit) return explicit;
  if (/^[a-zA-Z0-9]{9}$/u.test(search.get('short_id') ?? '') || search.get('purpose') === 'address')
    return 'create';
  return ['status', 'method', 'page'].some((key) => search.has(key)) ? 'records' : 'create';
}

export function claimViewHref(search: URLSearchParams, view: ClaimView): string {
  const query = new URLSearchParams(search);
  query.set('view', view);
  return `/dashboard/claims?${query}`;
}

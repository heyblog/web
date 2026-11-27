import { listPageNumber } from '../site-management/pagination.ts';

interface ReviewListFilters {
  readonly status: string;
  readonly action: string;
}

export function reviewListQuery(search: URLSearchParams) {
  const status = search.get('status') ?? 'PENDING';
  const action = search.get('action') ?? '';
  const page = listPageNumber(search);
  const query = new URLSearchParams({ page: String(page), page_size: '20' });
  if (status) query.set('status', status);
  if (action) query.set('action', action);
  return { status, action, page, query };
}

export function reviewListPath(filters: ReviewListFilters, page: number): string {
  const query = new URLSearchParams({
    status: filters.status,
    action: filters.action,
    page: String(page),
  });
  return `/management/site-submissions?${query}`;
}

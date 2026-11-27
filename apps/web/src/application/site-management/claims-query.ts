import { claimMethods, claimStatuses } from '../../api/site-management/claims.types.ts';

import { listPageNumber } from './pagination.ts';

export function claimListQuery(search: URLSearchParams, { management = false } = {}) {
  const page = listPageNumber(search);
  const query = new URLSearchParams({ page: String(page), page_size: '20' });
  const status = claimStatuses.find(
    (value) => value === (search.get('status') ?? (management ? 'PENDING' : null)),
  );
  const method = claimMethods.find(
    (value) => value === (search.get('method') ?? (management ? 'MANUAL' : null)),
  );
  if (status) query.set('status', status);
  if (method) query.set('method', method);
  return { page, query };
}

import type { SiteDirectoryQuery } from './site-directory.types.ts';

export function buildSiteDirectorySearchParams(query: SiteDirectoryQuery): URLSearchParams {
  const parameters = new URLSearchParams({
    page: String(query.page),
    q: query.q,
    feed: query.feed,
    status: query.status,
    sort: query.sort,
    order: query.order,
    seed: query.seed,
  });
  if (query.level1) parameters.set('level1', query.level1);
  if (query.level1 && query.level2) parameters.set('level2', query.level2);
  appendValues(parameters, 'tertiary', query.tertiary);
  appendValues(parameters, 'warning', query.warning);
  appendValues(parameters, 'technology', query.technology);
  appendValues(parameters, 'access', query.access);
  return parameters;
}

function appendValues(parameters: URLSearchParams, name: string, values: readonly string[]): void {
  for (const value of values) parameters.append(name, value);
}

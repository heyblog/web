import type {
  SiteDirectoryAccess,
  SiteDirectoryFeed,
  SiteDirectoryOrder,
  SiteDirectoryQuery,
  SiteDirectorySort,
  SiteDirectoryStatus,
} from './site-directory.models';

export function createDailyDirectorySeed(date = new Date()): string {
  const chinaDate = new Date(date.getTime() + 8 * 60 * 60 * 1000);
  const year = chinaDate.getUTCFullYear();
  const month = String(chinaDate.getUTCMonth() + 1).padStart(2, '0');
  const day = String(chinaDate.getUTCDate()).padStart(2, '0');
  return `site-directory:${year}-${month}-${day}`;
}

export function parseSiteDirectorySearchParams(parameters: URLSearchParams): SiteDirectoryQuery {
  const pageValue = Number.parseInt(parameters.get('page') ?? '1', 10);
  const feedValue = parameters.get('feed');
  const sortValue = parameters.get('sort');
  const orderValue = parameters.get('order');
  const statusValue = parameters.get('status');
  return {
    page: Number.isInteger(pageValue) && pageValue > 0 ? pageValue : 1,
    q: (parameters.get('q') ?? '').trim().slice(0, 100),
    primary: uniqueValues(parameters.getAll('primary')),
    secondary: uniqueValues(parameters.getAll('secondary')),
    warning: uniqueValues(parameters.getAll('warning')),
    technology: uniqueValues(parameters.getAll('technology')),
    access: uniqueValues(parameters.getAll('access')).filter(isDirectoryAccess),
    feed: isDirectoryFeed(feedValue) ? feedValue : 'any',
    status: isDirectoryStatus(statusValue) ? statusValue : 'normal',
    sort: isDirectorySort(sortValue) ? sortValue : 'random',
    order: isDirectoryOrder(orderValue) ? orderValue : 'desc',
    seed: validSeed(parameters.get('seed')) ?? createDailyDirectorySeed(),
  };
}

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
  appendValues(parameters, 'primary', query.primary);
  appendValues(parameters, 'secondary', query.secondary);
  appendValues(parameters, 'warning', query.warning);
  appendValues(parameters, 'technology', query.technology);
  appendValues(parameters, 'access', query.access);
  return parameters;
}

export function createDirectoryShuffleSeed(): string {
  return `site-directory:shuffle:${crypto.randomUUID()}`;
}

function appendValues(parameters: URLSearchParams, name: string, values: readonly string[]): void {
  for (const value of values) parameters.append(name, value);
}

function uniqueValues(values: readonly string[]): readonly string[] {
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))].slice(0, 20);
}

function validSeed(value: string | null): string | null {
  return value !== null && /^[A-Za-z0-9:_-]{1,96}$/.test(value) ? value : null;
}

function isDirectoryFeed(value: string | null): value is SiteDirectoryFeed {
  return value === 'any' || value === 'with' || value === 'without';
}

function isDirectorySort(value: string | null): value is SiteDirectorySort {
  return value === 'random' || value === 'joined' || value === 'updated';
}

function isDirectoryOrder(value: string | null): value is SiteDirectoryOrder {
  return value === 'asc' || value === 'desc';
}

function isDirectoryStatus(value: string | null): value is SiteDirectoryStatus {
  return value === 'normal' || value === 'abnormal';
}

function isDirectoryAccess(value: string): value is SiteDirectoryAccess {
  return value === 'ALL' || value === 'CN_ONLY' || value === 'GLOBAL_ONLY';
}

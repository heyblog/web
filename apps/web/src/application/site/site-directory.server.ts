import { getApiBaseUrl, readSessionUser } from '@/application/auth/auth.server';
import type {
  PagedResult,
  SiteArticleItem,
  SiteCheckItem,
  SiteDetail,
  SiteDirectoryMeta,
  SiteDirectoryPreference,
  SiteDirectoryResult,
  SiteGoResult,
} from '@/application/site/site-directory.models';
import {
  applyDirectoryPreference,
  buildSiteDirectorySearchParams,
  createDailyStableRandomSeed,
  hasExplicitDirectoryPreference,
  type SiteDirectoryQueryState,
} from '@/application/site/site-directory.shared';

type Envelope<T> = {
  ok: boolean;
  data: T;
};

async function fetchEnvelope<T>(
  request: Request | undefined,
  path: string,
): Promise<Envelope<T> | null> {
  try {
    const response = await fetch(`${getApiBaseUrl()}${path}`, {
      headers: {
        accept: 'application/json',
        ...(request?.headers.get('cookie')
          ? { cookie: request.headers.get('cookie') as string }
          : {}),
      },
    });

    if (!response.ok) {
      return null;
    }

    return (await response.json()) as Envelope<T>;
  } catch {
    return null;
  }
}

async function fetchRequiredEnvelope<T>(
  request: Request | undefined,
  path: string,
): Promise<Envelope<T> | null> {
  const response = await fetch(`${getApiBaseUrl()}${path}`, {
    headers: {
      accept: 'application/json',
      ...(request?.headers.get('cookie')
        ? { cookie: request.headers.get('cookie') as string }
        : {}),
    },
  });

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`request failed with status ${response.status}: ${path}`);
  }

  return (await response.json()) as Envelope<T>;
}

export async function fetchSiteDirectoryMeta(): Promise<SiteDirectoryMeta | null> {
  const payload = await fetchEnvelope<SiteDirectoryMeta>(undefined, '/api/public/sites/meta');
  return payload?.data ?? null;
}

export async function fetchSiteDirectoryPreference(
  request: Request,
): Promise<SiteDirectoryPreference | null> {
  const payload = await fetchEnvelope<SiteDirectoryPreference>(
    request,
    '/api/user/settings/site-directory',
  );

  return payload?.data ?? null;
}

export async function fetchSiteDirectory(
  query: SiteDirectoryQueryState,
): Promise<SiteDirectoryResult | null> {
  const payload = await fetchEnvelope<SiteDirectoryResult>(
    undefined,
    `/api/public/sites?${buildSiteDirectorySearchParams(query).toString()}`,
  );

  return payload?.data ?? null;
}

export async function fetchSiteRandom(search = ''): Promise<SiteGoResult | null> {
  const payload = await fetchEnvelope<SiteGoResult>(undefined, `/api/public/sites/random${search}`);
  return payload?.data ?? null;
}

async function canUseSiteDirectoryPreference(request: Request): Promise<boolean> {
  try {
    return Boolean(await readSessionUser(request));
  } catch {
    return false;
  }
}

export async function resolveInitialSiteDirectoryData(
  request: Request,
  url: URL,
  defaults: Pick<SiteDirectoryMeta['defaults'], 'pageSize' | 'statusMode'>,
): Promise<{
  result: SiteDirectoryResult | null;
  preference: SiteDirectoryPreference | null;
  canUsePreference: boolean;
}> {
  const params = url.searchParams;
  const canUsePreference = await canUseSiteDirectoryPreference(request);
  const preference = canUsePreference ? await fetchSiteDirectoryPreference(request) : null;
  const rawSort = params.get('sort');
  const sort: SiteDirectoryQueryState['sort'] =
    rawSort === 'updated' || rawSort === 'joined' || rawSort === 'visits' || rawSort === 'articles'
      ? rawSort
      : null;
  const initialQuery: SiteDirectoryQueryState = {
    q: params.get('q') ?? '',
    main: params.getAll('main'),
    sub: params.getAll('sub'),
    warning: params.getAll('warning'),
    program: params.getAll('program'),
    statusMode: defaults.statusMode,
    random: params.get('random') === 'off' ? false : true,
    sort,
    order: params.get('order') === 'asc' ? 'asc' : 'desc',
    randomSeed: params.get('randomSeed') || createDailyStableRandomSeed(),
    page: Math.max(1, Number(params.get('page') || 1)),
    pageSize: Math.max(12, Number(params.get('pageSize') || defaults.pageSize)),
  };

  const finalQuery = hasExplicitDirectoryPreference(params)
    ? initialQuery
    : applyDirectoryPreference(initialQuery, preference);

  return {
    result: await fetchSiteDirectory(finalQuery),
    preference,
    canUsePreference,
  };
}

export async function fetchSiteDetail(slug: string): Promise<SiteDetail | null> {
  const payload = await fetchRequiredEnvelope<SiteDetail>(undefined, `/api/public/sites/${slug}`);
  return payload?.data ?? null;
}

export async function fetchSiteArticles(
  slug: string,
  page: number,
  pageSize: number,
): Promise<PagedResult<SiteArticleItem> | null> {
  const payload = await fetchEnvelope<PagedResult<SiteArticleItem>>(
    undefined,
    `/api/public/sites/${slug}/articles?page=${page}&pageSize=${pageSize}`,
  );
  return payload?.data ?? null;
}

export async function fetchSiteChecks(
  slug: string,
  page: number,
  pageSize: number,
): Promise<PagedResult<SiteCheckItem> | null> {
  const payload = await fetchEnvelope<PagedResult<SiteCheckItem>>(
    undefined,
    `/api/public/sites/${slug}/checks?page=${page}&pageSize=${pageSize}`,
  );
  return payload?.data ?? null;
}

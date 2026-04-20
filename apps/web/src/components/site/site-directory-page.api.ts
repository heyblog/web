import type {
  SiteDirectoryMeta,
  SiteDirectoryPreference,
  SiteDirectoryResult,
} from '@/application/site/site-directory.models';
import type { SiteDirectoryQueryState } from '@/application/site/site-directory.shared';
import { buildSiteDirectorySearchParams } from '@/application/site/site-directory.shared';

type Envelope<T> = {
  ok: boolean;
  data: T;
};

export function resolveSiteDirectorySortSummary(
  sort: SiteDirectoryResult['query']['sort'],
): string {
  const labels = {
    updated: '更新时间',
    joined: '加入时间',
    visits: '访问数',
    articles: '文章数',
  };

  return sort ? (labels[sort] ?? '默认排序') : '默认排序';
}

export function resolveSiteDirectoryAccessSummary(access: string | undefined): string {
  const labels = {
    全球: '全球可访问',
    大陆: '仅中国大陆可访问',
    海外: '仅海外可访问',
  };

  return access ? (labels[access as keyof typeof labels] ?? '') : '';
}

export function syncSiteDirectoryUrl(nextQuery: SiteDirectoryQueryState) {
  if (typeof window === 'undefined') {
    return;
  }

  const params = buildSiteDirectorySearchParams(nextQuery);
  const query = params.toString();
  const target = `${window.location.pathname}${query ? `?${query}` : ''}`;

  window.history.replaceState({}, '', target);
}

export async function readSiteDirectoryPreference(
  canUsePreference: boolean,
): Promise<SiteDirectoryPreference | null> {
  if (!canUsePreference) {
    return null;
  }

  try {
    const response = await fetch('/api/site-directory/preferences', {
      headers: { accept: 'application/json' },
    });

    if (!response.ok) {
      return null;
    }

    const payload = (await response.json()) as Envelope<SiteDirectoryPreference>;
    return payload.data;
  } catch {
    return null;
  }
}

export async function persistSiteDirectoryPreference(
  canUsePreference: boolean,
  nextPreference: SiteDirectoryPreference,
): Promise<void> {
  if (!canUsePreference) {
    return;
  }

  try {
    await fetch('/api/site-directory/preferences', {
      method: 'PUT',
      headers: {
        accept: 'application/json',
        'content-type': 'application/json',
      },
      body: JSON.stringify(nextPreference),
    });
  } catch {
    // Best effort only.
  }
}

export async function requestSiteDirectory(
  nextQuery: SiteDirectoryQueryState,
): Promise<SiteDirectoryResult | null> {
  try {
    const response = await fetch(
      `/api/site-directory?${buildSiteDirectorySearchParams(nextQuery).toString()}`,
      {
        headers: { accept: 'application/json' },
      },
    );

    if (!response.ok) {
      return null;
    }

    const payload = (await response.json()) as Envelope<SiteDirectoryResult>;
    return payload.data;
  } catch {
    return null;
  }
}

export async function requestSiteDirectoryMeta(): Promise<SiteDirectoryMeta | null> {
  try {
    const response = await fetch('/api/site-directory/meta', {
      headers: { accept: 'application/json' },
    });

    if (!response.ok) {
      return null;
    }

    const payload = (await response.json()) as Envelope<SiteDirectoryMeta>;
    return payload.data;
  } catch {
    return null;
  }
}

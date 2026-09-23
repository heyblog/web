import type { SiteCardView } from '../home/home.shared.ts';

import { buildSiteGoHref, siteGoParameterMessage } from './site-go.shared.ts';

export type SiteGoSelection =
  { kind: 'success'; site: SiteCardView | null } | { kind: 'error'; message: string };

export async function previewRandomSite(
  level1: string,
  level2: string,
  signal: AbortSignal,
  fetcher: typeof fetch = fetch,
): Promise<SiteGoSelection> {
  try {
    const response = await fetcher(
      buildSiteGoHref(level1, level2).replace('/site/go', '/api/site-random'),
      {
        signal,
        cache: 'no-store',
        headers: { Accept: 'application/json' },
      },
    );
    if (response.status === 400) {
      const problem = (await response.json()) as { code?: string };
      return { kind: 'error', message: siteGoParameterMessage(problem.code) };
    }
    if (!response.ok) return { kind: 'error', message: '暂时无法获取博客，请稍后重试。' };
    const data = (await response.json()) as { site: SiteCardView | null };
    return { kind: 'success', site: data.site };
  } catch {
    return { kind: 'error', message: '暂时无法获取博客，请稍后重试。' };
  }
}

export async function copySiteGoLink(
  link: string,
  clipboard: Pick<Clipboard, 'writeText'> | undefined,
): Promise<boolean> {
  try {
    if (!clipboard) return false;
    await clipboard.writeText(link);
    return true;
  } catch {
    return false;
  }
}

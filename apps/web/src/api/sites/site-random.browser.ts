import { buildRandomSiteParameters } from './site-random.params.ts';

export function requestRandomSite(
  level1: string,
  level2: string,
  options: Readonly<{ signal: AbortSignal; fetch: typeof fetch }>,
): Promise<Response> {
  const query = buildRandomSiteParameters(level1, level2).toString();
  const fetcher = options.fetch;
  return fetcher(query ? `/api/web/public/site-random?${query}` : '/api/web/public/site-random', {
    signal: options.signal,
    cache: 'no-store',
    headers: { Accept: 'application/json' },
  });
}

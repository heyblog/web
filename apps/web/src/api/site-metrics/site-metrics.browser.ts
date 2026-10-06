import type { SiteImpressionEvent } from './site-metrics.types.ts';

export async function sendImpressions(
  events: readonly SiteImpressionEvent[],
  fetcher: typeof fetch = fetch,
): Promise<number> {
  try {
    const response = await fetcher('/api/site-metrics/impressions', {
      method: 'POST',
      credentials: 'same-origin',
      cache: 'no-store',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ events }),
      signal: AbortSignal.timeout(5_000),
    });
    if (response.status === 204) return 0;
    const retryAfter = Number(response.headers.get('Retry-After'));
    return response.status === 429 && retryAfter > 0 ? Math.min(retryAfter * 1_000, 60_000) : 2_000;
  } catch {
    return 2_000;
  }
}

export function beaconImpressions(
  events: readonly SiteImpressionEvent[],
  navigator: Pick<Navigator, 'sendBeacon'>,
  fetcher: typeof fetch = fetch,
): void {
  const body = new Blob([JSON.stringify({ events })], { type: 'application/json' });
  try {
    if (navigator.sendBeacon('/api/site-metrics/impressions', body)) return;
  } catch {
    // Some browser policies reject Beacon; keepalive is the unload fallback.
  }
  void fetcher('/api/site-metrics/impressions', {
    method: 'POST',
    credentials: 'same-origin',
    cache: 'no-store',
    headers: { 'Content-Type': 'application/json' },
    body,
    keepalive: true,
    signal: AbortSignal.timeout(5_000),
  }).catch(() => undefined);
}

import type { HomeView } from '@/application/home/home.shared';

export async function refreshHome(
  signal?: AbortSignal,
  fetcher: typeof fetch = fetch,
): Promise<HomeView> {
  const response = await fetcher('/api/home', {
    cache: 'no-store',
    headers: { Accept: 'application/json' },
    signal,
  });

  if (!response.ok) {
    throw new Error(`home refresh failed with status ${response.status}`);
  }

  return (await response.json()) as HomeView;
}

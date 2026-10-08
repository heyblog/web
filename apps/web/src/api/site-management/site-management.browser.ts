export async function accountRequest(
  path: string,
  init: Readonly<{
    method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
    body?: unknown;
    signal?: AbortSignal;
    management?: boolean;
    fetch?: typeof globalThis.fetch;
  }> = {},
): Promise<Response> {
  return (init.fetch ?? globalThis.fetch)(
    `/api/${init.management ? 'site-management' : 'account'}/${path}`,
    {
      method: init.method ?? 'GET',
      credentials: 'same-origin',
      cache: 'no-store',
      headers: { 'Content-Type': 'application/json' },
      ...(init.method && init.method !== 'GET' ? { body: JSON.stringify(init.body ?? {}) } : {}),
      signal: AbortSignal.any([AbortSignal.timeout(15_000), ...(init.signal ? [init.signal] : [])]),
    },
  );
}

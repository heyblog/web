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
  const timeoutMs = /^site-claim-attachments\/[^/]+\/complete$/u.test(path) ? 125_000 : 15_000;
  return (init.fetch ?? globalThis.fetch)(
    `/api/web/${init.management ? 'management' : 'account'}/${path}`,
    {
      method: init.method ?? 'GET',
      credentials: 'same-origin',
      cache: 'no-store',
      headers: { 'Content-Type': 'application/json' },
      ...(init.method && init.method !== 'GET' ? { body: JSON.stringify(init.body ?? {}) } : {}),
      signal: AbortSignal.any([
        AbortSignal.timeout(timeoutMs),
        ...(init.signal ? [init.signal] : []),
      ]),
    },
  );
}

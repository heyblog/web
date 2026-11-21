import { requestAuthAPI } from '../auth/auth.server.ts';

import { parseManagedSitePage } from './managed-sites.responses.ts';
import type { ManagedSitePage } from './managed-sites.types.ts';

export type ManagedSitesReadResult =
  | { readonly ok: true; readonly value: ManagedSitePage; readonly headers: Headers }
  | { readonly ok: false; readonly status: number; readonly headers: Headers };

export async function readManagedSites(
  request: Request,
  query: URLSearchParams,
): Promise<ManagedSitesReadResult> {
  const response = await requestAuthAPI(request, `/api/web/management/sites?${query}`);
  let payload: unknown;
  try {
    payload = await response.json();
  } catch (error) {
    if (!(error instanceof Error)) throw error;
    payload = null;
  }
  if (!response.ok) return { ok: false, status: response.status, headers: response.headers };
  const value = parseManagedSitePage(payload);
  return value
    ? { ok: true, value, headers: response.headers }
    : { ok: false, status: 502, headers: response.headers };
}

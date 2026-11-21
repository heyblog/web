import type { SessionUser } from '../../api/auth/auth.types.ts';
import { readManagedSites } from '../../api/managed-sites/managed-sites.server.ts';
import type { ManagedSitePage } from '../../api/managed-sites/managed-sites.types.ts';
import { readManagementActor } from '../../api/managed-users/managed-users.server.ts';

import { canManageSites } from './navigation.ts';
import { type ManagedSitesQuery, managedSitesQuery } from './sites.model.ts';

type SitesActor = Pick<SessionUser, 'display_name' | 'email' | 'role' | 'permissions'>;

type SitesPage =
  | { readonly kind: 'redirect'; readonly location: string; readonly headers: Headers }
  | {
      readonly kind: 'error';
      readonly actor: SitesActor | null;
      readonly query: ManagedSitesQuery;
      readonly status: number;
      readonly headers: Headers;
    }
  | {
      readonly kind: 'ready';
      readonly actor: SitesActor;
      readonly query: ManagedSitesQuery;
      readonly sites: ManagedSitePage;
      readonly headers: Headers;
    };

function copyCookies(source: Headers, target: Headers): void {
  for (const cookie of source.getSetCookie()) target.append('Set-Cookie', cookie);
}

export async function loadSitesPage(
  request: Request,
  resolvedActor?: SitesActor | null,
): Promise<SitesPage> {
  const headers = new Headers({ 'Cache-Control': 'private, no-store' });
  const url = new URL(request.url);
  const query = managedSitesQuery(url.searchParams);
  const login = `/login?next=${encodeURIComponent(url.pathname + url.search)}`;
  if (!request.headers.has('cookie')) return { kind: 'redirect', location: login, headers };
  const session = resolvedActor
    ? ({ ok: true, value: resolvedActor, headers: new Headers() } as const)
    : await readManagementActor(request);
  copyCookies(session.headers, headers);
  if (!session.ok) {
    if (session.status === 401 || session.status === 403)
      return {
        kind: 'redirect',
        location: session.status === 401 ? login : '/forbidden',
        headers,
      };
    return { kind: 'error', actor: null, query, status: 502, headers };
  }
  const actor = session.value;
  if (!canManageSites(actor)) return { kind: 'redirect', location: '/forbidden', headers };
  const upstreamQuery = new URLSearchParams({ page: String(query.page), page_size: '20' });
  if (query.q) upstreamQuery.set('q', query.q);
  if (query.visibility) upstreamQuery.set('visibility', query.visibility);
  const result = await readManagedSites(request, upstreamQuery);
  copyCookies(result.headers, headers);
  if (!result.ok) {
    if (result.status === 401 || result.status === 403)
      return {
        kind: 'redirect',
        location: result.status === 401 ? login : '/forbidden',
        headers,
      };
    return { kind: 'error', actor, query, status: 502, headers };
  }
  return {
    kind: 'ready',
    actor,
    query: { ...query, page: result.value.page },
    sites: result.value,
    headers,
  };
}

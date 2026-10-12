import {
  readManagedUser,
  readManagedUsers,
  readManagementActor,
} from '../../api/managed-users/managed-users.server.ts';
import type { ManagedUser } from '../../api/managed-users/managed-users.types.ts';

import { canManageUsers, usersQuery } from './users.model.ts';

type UsersPage =
  | { readonly kind: 'redirect'; readonly location: string; readonly headers: Headers }
  | {
      readonly kind: 'error';
      readonly actor: ManagedUser | null;
      readonly status: number;
      readonly headers: Headers;
    }
  | {
      readonly kind: 'ready';
      readonly actor: ManagedUser;
      readonly users: readonly ManagedUser[];
      readonly total: number;
      readonly page: number;
      readonly page_size: number;
      readonly headers: Headers;
    };

export async function loadUsersPage(
  request: Request,
  resolvedActor?: ManagedUser | null,
  userID?: string,
): Promise<UsersPage> {
  const headers = new Headers({ 'Cache-Control': 'private, no-store' });
  const url = new URL(request.url);
  const login = `/login?next=${encodeURIComponent(url.pathname + url.search)}`;
  if (!request.headers.has('cookie')) return { kind: 'redirect', location: login, headers };
  const session = resolvedActor
    ? ({ ok: true, value: resolvedActor, headers: new Headers() } as const)
    : await readManagementActor(request);
  for (const cookie of session.headers.getSetCookie()) headers.append('Set-Cookie', cookie);
  if (!session.ok) {
    if (session.status === 401 || session.status === 403)
      return {
        kind: 'redirect',
        location: session.status === 401 ? login : '/forbidden',
        headers,
      };
    return { kind: 'error', actor: null, status: 502, headers };
  }
  const actor = session.value;
  if (!canManageUsers(actor)) return { kind: 'redirect', location: '/forbidden', headers };
  const result = userID
    ? await readManagedUser(request, userID)
    : await readManagedUsers(request, usersQuery(url.searchParams));
  for (const cookie of result.headers.getSetCookie()) headers.append('Set-Cookie', cookie);
  if (!result.ok) {
    if (result.status === 401 || result.status === 403)
      return {
        kind: 'redirect',
        location: result.status === 401 ? login : '/forbidden',
        headers,
      };
    if (result.status === 404)
      return { kind: 'ready', actor, users: [], total: 0, page: 1, page_size: 20, headers };
    return { kind: 'error', actor, status: 502, headers };
  }
  const value =
    'users' in result.value
      ? result.value
      : { users: [result.value], total: 1, page: 1, page_size: 20 };
  return { kind: 'ready', actor, ...value, headers };
}

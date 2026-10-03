import {
  readManagedUsers,
  readManagementActor,
} from '../../api/managed-users/managed-users.server.ts';
import type { ManagedUser } from '../../api/managed-users/managed-users.types.ts';

import { canManageUsers } from './users.model.ts';

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
      readonly headers: Headers;
    };

export async function loadUsersPage(request: Request): Promise<UsersPage> {
  const headers = new Headers({ 'Cache-Control': 'private, no-store' });
  const url = new URL(request.url);
  const login = `/login?next=${encodeURIComponent(url.pathname + url.search)}`;
  if (!request.headers.has('cookie')) return { kind: 'redirect', location: login, headers };
  const session = await readManagementActor(request);
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
  const result = await readManagedUsers(request);
  for (const cookie of result.headers.getSetCookie()) headers.append('Set-Cookie', cookie);
  if (!result.ok) {
    if (result.status === 401 || result.status === 403)
      return {
        kind: 'redirect',
        location: result.status === 401 ? login : '/forbidden',
        headers,
      };
    return { kind: 'error', actor, status: 502, headers };
  }
  return { kind: 'ready', actor, users: result.value, headers };
}

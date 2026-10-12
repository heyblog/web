import { parseSessionResponse } from '../auth/auth.responses.ts';
import type { SessionUser } from '../auth/auth.types.ts';

import type { ProfileAction } from './profile.types.ts';

type ProfileResult =
  | { readonly ok: true; readonly user: SessionUser }
  | { readonly ok: false; readonly code: string; readonly uncertain: boolean };

export async function saveProfile(action: ProfileAction): Promise<ProfileResult> {
  const routes = { confirm: 'profile/confirm', display: 'profile', username: 'username' } as const;
  const { kind, ...body } = action;
  try {
    const response = await fetch(`/api/web/account/${routes[kind]}`, {
      method: kind === 'display' ? 'PATCH' : 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      credentials: 'same-origin',
      cache: 'no-store',
      redirect: 'error',
      signal: AbortSignal.timeout(12_000),
      body: JSON.stringify(body),
    });
    const payload: unknown = await response.json();
    if (response.ok) {
      const user = parseSessionResponse(payload);
      return user ? { ok: true, user } : { ok: false, code: 'request_failed', uncertain: true };
    }
    const code =
      typeof payload === 'object' &&
      payload !== null &&
      'code' in payload &&
      typeof payload.code === 'string'
        ? payload.code
        : 'request_failed';
    return { ok: false, code, uncertain: response.status >= 500 };
  } catch {
    return { ok: false, code: 'request_failed', uncertain: true };
  }
}

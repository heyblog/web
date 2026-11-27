import { parseSessionResponse } from '../auth/auth.responses.ts';
import type { SessionUser } from '../auth/auth.types.ts';

import {
  type EmailChangeSide,
  type EmailChangeStatus,
  parseEmailChangeStatus,
} from './email-change.types.ts';

type EmailChangeMutation =
  | { readonly path: ''; readonly method: 'POST'; readonly body: { readonly email: string } }
  | { readonly path: ''; readonly method: 'DELETE'; readonly id: string }
  | {
      readonly path: '/send-code';
      readonly method: 'POST';
      readonly id: string;
      readonly body: { readonly side: EmailChangeSide };
    }
  | {
      readonly path: '/verify-code';
      readonly method: 'POST';
      readonly id: string;
      readonly body: { readonly side: EmailChangeSide; readonly code: string };
    }
  | {
      readonly path: '/verify-password';
      readonly method: 'POST';
      readonly id: string;
      readonly body: { readonly password: string };
    };

type CompleteEmailChange = {
  readonly path: '/complete';
  readonly method: 'POST';
  readonly id: string;
  readonly body: Readonly<Record<string, never>>;
};

type EmailChangeResult<T> =
  | { readonly ok: true; readonly value: T }
  | { readonly ok: false; readonly code: string; readonly uncertain: boolean };

async function requestEmailChange<T>(
  action: EmailChangeMutation | CompleteEmailChange | undefined,
  parse: (value: unknown) => T | null,
): Promise<EmailChangeResult<T>> {
  const headers = new Headers({ Accept: 'application/json' });
  const body = action && 'body' in action ? JSON.stringify(action.body) : undefined;
  if (body) headers.set('Content-Type', 'application/json');
  if (action && 'id' in action) headers.set('X-HeyBlog-Email-Change-ID', action.id);
  try {
    const response = await fetch('/api/web/account/email-change' + (action?.path ?? ''), {
      method: action?.method ?? 'GET',
      headers,
      credentials: 'same-origin',
      cache: 'no-store',
      redirect: 'error',
      signal: AbortSignal.timeout(12_000),
      body,
    });
    const payload: unknown = await response.json();
    if (!response.ok) {
      const code =
        typeof payload === 'object' &&
        payload !== null &&
        'code' in payload &&
        typeof payload.code === 'string'
          ? payload.code
          : 'request_failed';
      return {
        ok: false,
        code: response.status === 401 ? 'session_expired' : code,
        uncertain: response.status >= 500,
      };
    }
    const value = parse(payload);
    if (value !== null) return { ok: true, value };
  } catch (error) {
    if (!(error instanceof Error)) throw error;
  }
  return { ok: false, code: 'request_failed', uncertain: true };
}

export function readEmailChange(): Promise<EmailChangeResult<EmailChangeStatus>> {
  return requestEmailChange(undefined, parseEmailChangeStatus);
}

export function updateEmailChange(
  action: EmailChangeMutation,
): Promise<EmailChangeResult<EmailChangeStatus>> {
  return requestEmailChange(action, parseEmailChangeStatus);
}

export function completeEmailChange(id: string): Promise<EmailChangeResult<SessionUser>> {
  return requestEmailChange(
    { path: '/complete', method: 'POST', id, body: {} },
    parseSessionResponse,
  );
}

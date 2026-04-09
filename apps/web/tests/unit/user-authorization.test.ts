import { describe, expect, it } from 'vitest';

import {
  buildRoleManagementPath,
  createManagementActionJsonResponse,
  isValidManagedUserId,
  readManagedUserFromEnvelope,
  readManagementApiErrorMessage,
  readRoleManagementIntent,
} from '@/application/management/user-authorization.server';

describe('management user authorization helpers', () => {
  it('validates managed user ids and role intents', () => {
    expect(isValidManagedUserId('11111111-1111-4111-8111-111111111111')).toBe(true);
    expect(isValidManagedUserId('not-a-uuid')).toBe(false);
    expect(readRoleManagementIntent('grant-admin')).toBe('grant-admin');
    expect(readRoleManagementIntent('revoke-admin')).toBe('revoke-admin');
    expect(readRoleManagementIntent('invalid-intent')).toBeNull();
  });

  it('builds role paths', () => {
    expect(buildRoleManagementPath('grant-admin', '11111111-1111-4111-8111-111111111111')).toBe(
      '/api/management/users/11111111-1111-4111-8111-111111111111/grant-admin',
    );
  });

  it('reads API error message with payload-first fallback behavior', async () => {
    const withPayload = new Response(
      JSON.stringify({
        error: {
          message: '  Permission denied for this action.  ',
        },
      }),
      {
        status: 403,
      },
    );
    const withoutPayload = new Response('plain error', {
      status: 404,
    });

    expect(await readManagementApiErrorMessage(withPayload, 'fallback')).toBe(
      'Permission denied for this action.',
    );
    expect(await readManagementApiErrorMessage(withoutPayload, 'fallback')).toBe(
      '目标用户不存在或已被删除。',
    );
  });

  it('wraps json payloads', async () => {
    const response = createManagementActionJsonResponse(
      {
        ok: true,
        data: {
          id: 'user-1',
        },
      },
      {
        status: 201,
      },
    );

    expect(response.status).toBe(201);
    expect(await readManagedUserFromEnvelope<{ id: string }>(response)).toEqual({
      id: 'user-1',
    });
  });
});

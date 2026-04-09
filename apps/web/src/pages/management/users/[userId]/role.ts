import type { APIRoute } from 'astro';

import type { ManagedUserSnapshot } from '@/application/management/user-authorization.browser';
import {
  buildRoleManagementPath,
  createManagementActionJsonResponse,
  getManagementUsersLoginRedirect,
  isValidManagedUserId,
  proxyManagementRequest,
  readManagedUserFromEnvelope,
  readManagementApiErrorMessage,
  readRoleManagementIntent,
} from '@/application/management/user-authorization.server';

export const prerender = false;

const INVALID_REQUEST_MESSAGE = '角色授权请求无效，请刷新后重试。';
const UPDATE_FAILURE_MESSAGE = '角色授权更新失败，请稍后重试。';
const STATUS_BY_INTENT = {
  'grant-admin': 'role_granted',
  'revoke-admin': 'role_revoked',
} as const;

export const POST: APIRoute = async ({ request, params }) => {
  const formData = await request.formData();
  const userId = params.userId;
  const intent = readRoleManagementIntent(formData.get('intent'));

  if (!isValidManagedUserId(userId) || !intent) {
    return createManagementActionJsonResponse(
      {
        ok: false,
        code: 'invalid_role_request',
        target: userId ?? null,
        redirect: null,
        message: INVALID_REQUEST_MESSAGE,
        data: null,
      },
      { status: 400 },
    );
  }

  const response = await proxyManagementRequest(request, buildRoleManagementPath(intent, userId), {
    method: 'POST',
  });

  if (response.status === 401) {
    const redirectLocation = getManagementUsersLoginRedirect();

    return createManagementActionJsonResponse(
      {
        ok: false,
        code: 'unauthorized',
        target: userId,
        redirect: redirectLocation,
        message: '登录状态已过期，请重新登录。',
        data: null,
      },
      { status: 401 },
    );
  }

  if (!response.ok) {
    const message = await readManagementApiErrorMessage(response, UPDATE_FAILURE_MESSAGE);

    return createManagementActionJsonResponse(
      {
        ok: false,
        code: 'role_update_failed',
        target: userId,
        redirect: null,
        message,
        data: null,
      },
      { status: response.status },
    );
  }

  const data = await readManagedUserFromEnvelope<ManagedUserSnapshot>(response);

  if (!data) {
    return createManagementActionJsonResponse(
      {
        ok: false,
        code: 'request_failed',
        target: userId,
        redirect: null,
        message: UPDATE_FAILURE_MESSAGE,
        data: null,
      },
      { status: 502 },
    );
  }

  const status = STATUS_BY_INTENT[intent];

  return createManagementActionJsonResponse({
    ok: true,
    status,
    target: userId,
    code: undefined,
    redirect: null,
    message: '',
    data,
  });
};

import type { APIRoute } from 'astro';

import { MANAGEMENT_NAV_ITEMS, type ManagementPermissionKey } from '@/application/auth/auth.guard';
import type { ManagedUserSnapshot } from '@/application/management/user-authorization.browser';
import {
  createManagementActionJsonResponse,
  getManagementUsersLoginRedirect,
  isValidManagedUserId,
  proxyManagementRequest,
  readManagedUserFromEnvelope,
  readManagementApiErrorMessage,
} from '@/application/management/user-authorization.server';

export const prerender = false;

const INVALID_REQUEST_MESSAGE = '模块权限请求无效，请刷新后重试。';
const UPDATE_FAILURE_MESSAGE = '模块权限更新失败，请稍后重试。';
const VALID_PERMISSION_SET = new Set(MANAGEMENT_NAV_ITEMS.map((item) => item.permission));

const isValidManagementPermission = (value: string): value is ManagementPermissionKey =>
  VALID_PERMISSION_SET.has(value as ManagementPermissionKey);

export const POST: APIRoute = async ({ request, params }) => {
  const userId = params.userId;

  if (!isValidManagedUserId(userId)) {
    return createManagementActionJsonResponse(
      {
        ok: false,
        code: 'invalid_permissions_request',
        target: userId ?? null,
        redirect: null,
        message: INVALID_REQUEST_MESSAGE,
        data: null,
      },
      { status: 400 },
    );
  }

  const formData = await request.formData();
  const rawPermissions = formData.getAll('permissions');
  const permissions: ManagementPermissionKey[] = [];

  for (const value of rawPermissions) {
    if (typeof value !== 'string' || !isValidManagementPermission(value)) {
      return createManagementActionJsonResponse(
        {
          ok: false,
          code: 'invalid_permissions_request',
          target: userId,
          redirect: null,
          message: INVALID_REQUEST_MESSAGE,
          data: null,
        },
        { status: 400 },
      );
    }

    if (!permissions.includes(value)) {
      permissions.push(value);
    }
  }

  const response = await proxyManagementRequest(
    request,
    `/api/management/users/${userId}/permissions`,
    {
      method: 'PUT',
      headers: {
        'content-type': 'application/json',
      },
      body: JSON.stringify({
        permissions,
      }),
    },
  );

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
        code: 'permission_update_failed',
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

  return createManagementActionJsonResponse({
    ok: true,
    status: 'permissions_updated',
    target: userId,
    code: undefined,
    redirect: null,
    message: '',
    data,
  });
};

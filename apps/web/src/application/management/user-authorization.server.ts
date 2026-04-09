import { getApiBaseUrl } from '@/application/auth/auth.server';

const USER_ID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const MESSAGE_MAX_LENGTH = 160;
const REDIRECT_LOGIN = '/login?next=%2Fmanagement%2Fusers';

type ErrorPayload = {
  code?: string;
  message?: string;
  error?: {
    code?: string;
    message?: string;
  };
};

export type RoleManagementIntent = 'grant-admin' | 'revoke-admin';

const STATUS_FALLBACK_MESSAGES: Record<number, string> = {
  400: '请求参数无效，请刷新后重试。',
  401: '登录状态已过期，请重新登录。',
  403: '当前账号无权执行这次授权操作。',
  404: '目标用户不存在或已被删除。',
  409: '目标授权状态已变化，请刷新后重试。',
};

const normalizeFeedbackMessage = (value: string): string =>
  value.replace(/\s+/g, ' ').trim().slice(0, MESSAGE_MAX_LENGTH);

const pickMessageFromPayload = (payload: ErrorPayload | null): string | null => {
  const message = payload?.error?.message ?? payload?.message;

  if (!message?.trim()) {
    return null;
  }

  return normalizeFeedbackMessage(message);
};

export const isValidManagedUserId = (value: string | null | undefined): value is string =>
  typeof value === 'string' && USER_ID_PATTERN.test(value);

export const readRoleManagementIntent = (
  value: FormDataEntryValue | null,
): RoleManagementIntent | null => {
  if (value === 'grant-admin' || value === 'revoke-admin') {
    return value;
  }

  return null;
};

export const buildRoleManagementPath = (intent: RoleManagementIntent, userId: string): string =>
  intent === 'grant-admin'
    ? `/api/management/users/${userId}/grant-admin`
    : `/api/management/users/${userId}/revoke-admin`;

export const getManagementUsersLoginRedirect = (): string => REDIRECT_LOGIN;

export const proxyManagementRequest = (
  request: Request,
  path: string,
  init: RequestInit,
): Promise<Response> =>
  fetch(`${getApiBaseUrl()}${path}`, {
    ...init,
    headers: {
      accept: 'application/json',
      cookie: request.headers.get('cookie') ?? '',
      ...(init.headers ?? {}),
    },
  });

export const readManagementApiErrorMessage = async (
  response: Response,
  fallbackMessage: string,
): Promise<string> => {
  try {
    const payload = (await response.json()) as ErrorPayload;
    const parsedMessage = pickMessageFromPayload(payload);

    if (parsedMessage) {
      return parsedMessage;
    }
  } catch {
    // Ignore parse errors and fallback to default status/message mapping.
  }

  return STATUS_FALLBACK_MESSAGES[response.status] ?? fallbackMessage;
};

export const createManagementActionJsonResponse = (
  payload: {
    ok: boolean;
    status?: string | null;
    target?: string | null;
    message?: string | null;
    code?: string | null;
    redirect?: string | null;
    data?: unknown;
  },
  init: ResponseInit = {},
): Response =>
  new Response(JSON.stringify(payload), {
    status: init.status ?? 200,
    headers: {
      'content-type': 'application/json; charset=utf-8',
      ...(init.headers ?? {}),
    },
  });

export const readManagedUserFromEnvelope = async <T>(response: Response): Promise<T | null> => {
  try {
    const payload = (await response.json()) as {
      data?: T;
    };

    return payload?.data ?? null;
  } catch {
    return null;
  }
};

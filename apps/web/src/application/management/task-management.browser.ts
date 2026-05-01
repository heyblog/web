import { getConfiguredApiBaseUrl } from '@/application/api/api-env';

const DEFAULT_FAILURE_MESSAGE = '请求未完成，请稍后重试。';

const isCrossOriginApiBaseUrl = (value: string): boolean => {
  if (typeof window === 'undefined') {
    return false;
  }

  try {
    return new URL(value).origin !== window.location.origin;
  } catch {
    return false;
  }
};

const resolveApiBaseUrl = (): string => {
  const apiBaseUrl = getConfiguredApiBaseUrl();
  if (!apiBaseUrl) {
    return '';
  }

  return isCrossOriginApiBaseUrl(apiBaseUrl) ? apiBaseUrl : '';
};

export const buildTaskManagementApiPath = (path: string): string =>
  (() => {
    const apiBaseUrl = resolveApiBaseUrl();
    return apiBaseUrl ? `${apiBaseUrl}${path}` : path;
  })();

const normalizeMessage = (value: unknown, fallback: string): string => {
  if (typeof value !== 'string') {
    return fallback;
  }

  const normalized = value.replace(/\s+/g, ' ').trim();
  return normalized || fallback;
};

export interface TaskManagementActionResult<T = unknown> {
  ok: boolean;
  statusCode: number;
  code: string;
  message: string;
  data: T | null;
}

const parseTaskManagementResponse = async <T>(
  response: Response,
): Promise<TaskManagementActionResult<T>> => {
  const payload = (await response.json().catch(() => null)) as {
    ok?: boolean;
    data?: T;
    error?: {
      code?: string;
      message?: string;
    };
  } | null;

  if (payload?.ok === true) {
    return {
      ok: true,
      statusCode: response.status,
      code: '',
      message: '',
      data: (payload.data ?? null) as T | null,
    };
  }

  return {
    ok: false,
    statusCode: response.status,
    code: payload?.error?.code?.trim() || `http_${response.status}`,
    message: normalizeMessage(payload?.error?.message, DEFAULT_FAILURE_MESSAGE),
    data: null,
  };
};

const requestTaskApi = async <T>(
  path: string,
  init: {
    method: 'GET' | 'POST' | 'PUT' | 'DELETE';
    body?: unknown;
  },
): Promise<TaskManagementActionResult<T>> => {
  const headers = new Headers({
    accept: 'application/json',
  });

  const requestInit: RequestInit = {
    method: init.method,
    credentials: 'include',
    headers,
  };

  if (init.body !== undefined) {
    headers.set('content-type', 'application/json');
    requestInit.body = JSON.stringify(init.body);
  }

  const response = await fetch(buildTaskManagementApiPath(path), requestInit);
  return parseTaskManagementResponse<T>(response);
};

const buildQueryString = (query: Record<string, string | number | undefined>): string => {
  const search = new URLSearchParams();

  for (const [key, value] of Object.entries(query)) {
    if (value === undefined) {
      continue;
    }

    const normalized = String(value).trim();
    if (!normalized) {
      continue;
    }

    search.set(key, normalized);
  }

  const serialized = search.toString();
  return serialized ? `?${serialized}` : '';
};

export const saveTaskScheduleAction = (
  body: Record<string, unknown>,
): Promise<TaskManagementActionResult<Record<string, unknown>>> =>
  requestTaskApi('/api/management/tasks/schedules', {
    method: 'POST',
    body,
  });

export const toggleTaskScheduleAction = (
  scheduleId: string,
): Promise<TaskManagementActionResult<Record<string, unknown>>> =>
  requestTaskApi(`/api/management/tasks/schedules/${scheduleId}/toggle`, {
    method: 'POST',
  });

export const runTaskScheduleAction = (
  scheduleId: string,
): Promise<TaskManagementActionResult<Record<string, unknown>>> =>
  requestTaskApi(`/api/management/tasks/schedules/${scheduleId}/run`, {
    method: 'POST',
  });

export const deleteTaskScheduleAction = (
  scheduleId: string,
): Promise<TaskManagementActionResult<{ id: string }>> =>
  requestTaskApi(`/api/management/tasks/schedules/${scheduleId}`, {
    method: 'DELETE',
  });

export const fetchTaskRequestConfigsAction = (): Promise<
  TaskManagementActionResult<Array<Record<string, unknown>>>
> =>
  requestTaskApi('/api/management/tasks/request-configs', {
    method: 'GET',
  });

export const saveTaskRequestConfigAction = (
  body: Record<string, unknown>,
): Promise<TaskManagementActionResult<Record<string, unknown>>> =>
  requestTaskApi('/api/management/tasks/request-configs', {
    method: 'POST',
    body,
  });

export const toggleTaskRequestConfigAction = (
  configId: string,
): Promise<TaskManagementActionResult<Record<string, unknown>>> =>
  requestTaskApi(`/api/management/tasks/request-configs/${configId}/toggle`, {
    method: 'POST',
  });

export const deleteTaskRequestConfigAction = (
  configId: string,
): Promise<TaskManagementActionResult<{ id: string }>> =>
  requestTaskApi(`/api/management/tasks/request-configs/${configId}`, {
    method: 'DELETE',
  });

export const runManualSiteCheckAction = (
  body: Record<string, unknown>,
): Promise<TaskManagementActionResult<Record<string, unknown>>> =>
  requestTaskApi('/api/management/tasks/manual/site-check', {
    method: 'POST',
    body,
  });

export const runManualRSSFetchAction = (
  body: Record<string, unknown>,
): Promise<TaskManagementActionResult<Record<string, unknown>>> =>
  requestTaskApi('/api/management/tasks/manual/rss-fetch', {
    method: 'POST',
    body,
  });

export const cancelTaskJobAction = (
  jobId: string,
): Promise<TaskManagementActionResult<Record<string, unknown>>> =>
  requestTaskApi(`/api/management/tasks/jobs/${jobId}/cancel`, {
    method: 'POST',
  });

export const requeueTaskJobAction = (
  jobId: string,
): Promise<TaskManagementActionResult<Record<string, unknown>>> =>
  requestTaskApi(`/api/management/tasks/jobs/${jobId}/requeue`, {
    method: 'POST',
  });

export const deleteTaskJobAction = (
  jobId: string,
): Promise<TaskManagementActionResult<{ id: string }>> =>
  requestTaskApi(`/api/management/tasks/jobs/${jobId}`, {
    method: 'DELETE',
  });

export const fetchTaskOverviewAction = (
  filters: Record<string, string | number | undefined>,
): Promise<
  TaskManagementActionResult<{
    schedules: Array<Record<string, unknown>>;
    jobs: Array<Record<string, unknown>>;
  }>
> =>
  requestTaskApi(`/api/management/tasks/overview${buildQueryString(filters)}`, {
    method: 'GET',
  });

export const fetchTaskJobDetailAction = (
  jobId: string,
): Promise<TaskManagementActionResult<Record<string, unknown>>> =>
  requestTaskApi(`/api/management/tasks/jobs/${jobId}`, {
    method: 'GET',
  });

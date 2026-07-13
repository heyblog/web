import { proxyUpstreamBody } from '@/application/shared/upstream-proxy.server';

type TaskManagementProxyMethod = 'GET' | 'POST' | 'PUT' | 'DELETE';

const JSON_CONTENT_TYPE = 'application/json; charset=utf-8';
const TASK_MANAGEMENT_PROXY_FAILURE = {
  fallbackMessage: '任务管理请求失败。',
  fallbackCode: 'TASK_MANAGEMENT_PROXY_FAILED',
  fallbackContentType: JSON_CONTENT_TYPE,
};

function createTaskManagementProxyNotFoundResponse(): Response {
  return new Response(
    JSON.stringify({
      ok: false,
      error: {
        code: 'TASK_MANAGEMENT_API_NOT_FOUND',
        message: 'Task management API route was not found.',
      },
    }),
    {
      status: 404,
      headers: {
        'cache-control': 'no-store',
        'content-type': JSON_CONTENT_TYPE,
      },
    },
  );
}

function normalizeTaskManagementProxyPath(path: string): string | null {
  const normalized = path.trim().replace(/^\/+|\/+$/g, '');
  if (!normalized) {
    return null;
  }

  const segments = normalized.split('/');
  if (segments.some((segment) => !segment || segment === '.' || segment === '..')) {
    return null;
  }

  return normalized;
}

function isAllowedTaskManagementProxyPath(
  path: string,
  method: TaskManagementProxyMethod,
): boolean {
  const segments = path.split('/');

  if (method === 'GET' && (path === 'catalog' || path === 'overview')) {
    return true;
  }

  if (path === 'schedules') {
    return method === 'POST';
  }

  if (segments[0] === 'schedules' && segments.length === 2) {
    return method === 'DELETE';
  }

  if (
    segments[0] === 'schedules' &&
    segments.length === 3 &&
    (segments[2] === 'toggle' || segments[2] === 'run')
  ) {
    return method === 'POST';
  }

  if (path === 'request-configs') {
    return method === 'GET' || method === 'POST';
  }

  if (segments[0] === 'request-configs' && segments.length === 2) {
    return method === 'GET' || method === 'DELETE';
  }

  if (segments[0] === 'request-configs' && segments.length === 3 && segments[2] === 'toggle') {
    return method === 'POST';
  }

  if (path === 'manual/site-check' || path === 'manual/rss-fetch') {
    return method === 'POST';
  }

  if (segments[0] === 'jobs' && segments.length === 2) {
    return method === 'GET' || method === 'DELETE';
  }

  if (
    segments[0] === 'jobs' &&
    segments.length === 3 &&
    (segments[2] === 'cancel' || segments[2] === 'requeue')
  ) {
    return method === 'POST';
  }

  return false;
}

async function buildTaskManagementProxyInit(request: Request): Promise<RequestInit> {
  const headers = new Headers();
  const accept = request.headers.get('accept');
  const contentType = request.headers.get('content-type');
  const method = request.method.toUpperCase();

  if (accept) {
    headers.set('accept', accept);
  }

  if (contentType && method !== 'GET') {
    headers.set('content-type', contentType);
  }

  const init: RequestInit = {
    method,
    headers,
  };

  if (method !== 'GET' && method !== 'HEAD') {
    init.body = await request.arrayBuffer();
  }

  return init;
}

export async function handleTaskManagementStreamRequest(request?: Request): Promise<Response> {
  void request;

  return new Response(
    JSON.stringify({
      ok: false,
      error: {
        code: 'TASK_STREAM_REMOVED',
        message: 'Task realtime stream has been removed from the refactored task system.',
      },
    }),
    {
      status: 410,
      headers: {
        'content-type': 'application/json; charset=utf-8',
      },
    },
  );
}

export async function handleTaskManagementApiProxyRequest(
  path: string,
  request: Request,
): Promise<Response> {
  const method = request.method.toUpperCase() as TaskManagementProxyMethod;
  const normalizedPath = normalizeTaskManagementProxyPath(path);

  if (!normalizedPath || !isAllowedTaskManagementProxyPath(normalizedPath, method)) {
    return createTaskManagementProxyNotFoundResponse();
  }

  const search = new URL(request.url).search;

  return proxyUpstreamBody(
    `/api/management/tasks/${normalizedPath}${search}`,
    await buildTaskManagementProxyInit(request),
    {
      request,
      ...TASK_MANAGEMENT_PROXY_FAILURE,
    },
  );
}

export async function handleTaskJobExportRequest(
  jobId: string,
  request?: Request,
): Promise<Response> {
  if (!jobId.trim()) {
    return new Response(
      JSON.stringify({
        ok: false,
        error: {
          code: 'INVALID_JOB_ID',
          message: 'jobId is required.',
        },
      }),
      {
        status: 400,
        headers: {
          'content-type': 'application/json; charset=utf-8',
        },
      },
    );
  }

  return proxyUpstreamBody(
    `/api/management/tasks/jobs/${jobId}/export.xls`,
    { method: 'GET' },
    {
      request,
      fallbackMessage: '任务导出失败。',
      fallbackCode: 'TASK_EXPORT_FAILED',
      fallbackContentType: 'application/json; charset=utf-8',
    },
  );
}

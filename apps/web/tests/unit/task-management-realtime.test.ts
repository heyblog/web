import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  buildTaskManagementApiPath,
  runTaskScheduleAction,
} from '@/application/management/task-management.browser';
import {
  handleTaskManagementApiProxyRequest,
  handleTaskManagementStreamRequest,
} from '@/application/management/task-management.server-handler';

import { getApiBaseUrl, getWebBaseUrl } from '../setup/env';

describe('task management transport helpers', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('builds same-origin management api paths by default', () => {
    expect(buildTaskManagementApiPath('/api/management/tasks/catalog')).toBe(
      '/api/management/tasks/catalog',
    );
  });

  it('sends JSON mutation requests even when the action has no payload', async () => {
    const fetchMock = vi.fn().mockResolvedValue(Response.json({ ok: true, data: { id: 'job-1' } }));
    vi.stubGlobal('fetch', fetchMock);

    await runTaskScheduleAction('019df246-b566-72b8-a45c-008901e0cd91');

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/management/tasks/schedules/019df246-b566-72b8-a45c-008901e0cd91/run',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include',
        headers: expect.any(Headers),
        body: '{}',
      }),
    );
    const headers = fetchMock.mock.calls[0]?.[1]?.headers as Headers;
    expect(headers.get('content-type')).toBe('application/json');
  });

  it('returns 410 for the removed task stream endpoint', async () => {
    const response = await handleTaskManagementStreamRequest(
      new Request('http://127.0.0.1:9101/api/management/tasks/stream?job_id=job-1'),
    );

    expect(response.status).toBe(410);
    expect(response.headers.get('content-type')).toContain('application/json');
    await expect(response.json()).resolves.toEqual({
      ok: false,
      error: {
        code: 'TASK_STREAM_REMOVED',
        message: 'Task realtime stream has been removed from the refactored task system.',
      },
    });
  });

  it('proxies request config reads to the upstream API', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      Response.json({
        ok: true,
        data: [],
      }),
    );
    vi.stubGlobal('fetch', fetchMock);

    const response = await handleTaskManagementApiProxyRequest(
      'request-configs',
      new Request(`${getWebBaseUrl()}/api/management/tasks/request-configs`, {
        headers: {
          cookie: 'zhblogs_session=session-1',
        },
      }),
    );

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({ ok: true, data: [] });
    expect(fetchMock).toHaveBeenCalledWith(
      `${getApiBaseUrl()}/api/management/tasks/request-configs`,
      expect.objectContaining({
        method: 'GET',
        headers: expect.any(Headers),
      }),
    );
    const headers = fetchMock.mock.calls[0]?.[1]?.headers as Headers;
    expect(headers.get('cookie')).toBe('zhblogs_session=session-1');
  });

  it('proxies task catalog reads to the upstream API', async () => {
    const fetchMock = vi.fn().mockResolvedValue(Response.json({ ok: true, data: {} }));
    vi.stubGlobal('fetch', fetchMock);

    await handleTaskManagementApiProxyRequest(
      'catalog',
      new Request(`${getWebBaseUrl()}/api/management/tasks/catalog`),
    );

    expect(fetchMock).toHaveBeenCalledWith(
      `${getApiBaseUrl()}/api/management/tasks/catalog`,
      expect.objectContaining({
        method: 'GET',
      }),
    );
  });

  it('preserves query strings for overview requests', async () => {
    const fetchMock = vi.fn().mockResolvedValue(Response.json({ ok: true, data: {} }));
    vi.stubGlobal('fetch', fetchMock);

    await handleTaskManagementApiProxyRequest(
      'overview',
      new Request(`${getWebBaseUrl()}/api/management/tasks/overview?status=FAILED&limit=20`),
    );

    expect(fetchMock).toHaveBeenCalledWith(
      `${getApiBaseUrl()}/api/management/tasks/overview?status=FAILED&limit=20`,
      expect.objectContaining({
        method: 'GET',
      }),
    );
  });

  it('proxies JSON mutation bodies', async () => {
    const fetchMock = vi.fn().mockResolvedValue(Response.json({ ok: true, data: { id: 'cfg-1' } }));
    vi.stubGlobal('fetch', fetchMock);

    await handleTaskManagementApiProxyRequest(
      'request-configs',
      new Request(`${getWebBaseUrl()}/api/management/tasks/request-configs`, {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
        },
        body: JSON.stringify({ name: 'Default crawler' }),
      }),
    );

    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(init.method).toBe('POST');
    expect((init.headers as Headers).get('content-type')).toBe('application/json');
    await expect(new Response(init.body).json()).resolves.toEqual({ name: 'Default crawler' });
  });

  it('rejects unknown task management proxy paths', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);

    const response = await handleTaskManagementApiProxyRequest(
      '../request-configs',
      new Request(`${getWebBaseUrl()}/api/management/tasks/../request-configs`),
    );

    expect(response.status).toBe(404);
    expect(fetchMock).not.toHaveBeenCalled();
    await expect(response.json()).resolves.toEqual({
      ok: false,
      error: {
        code: 'TASK_MANAGEMENT_API_NOT_FOUND',
        message: 'Task management API route was not found.',
      },
    });
  });
});

import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  readAnnouncementArchive,
  readCurrentAnnouncement,
} from '@/application/announcement/announcement.server';
import { POST as saveAnnouncementPost } from '@/pages/management/announcements/save';

import { getApiBaseUrl, getWebBaseUrl } from '../setup/env';

describe('announcement server readers', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('reads current announcement payload', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              ok: true,
              data: {
                id: 'announcement-1',
                title: 'Current',
                content: 'Current body',
                publishTime: '2026-03-29T10:00:00.000Z',
              },
            }),
            {
              status: 200,
              headers: {
                'content-type': 'application/json',
              },
            },
          ),
      ),
    );

    await expect(readCurrentAnnouncement()).resolves.toEqual({
      id: 'announcement-1',
      title: 'Current',
      content: 'Current body',
      publishTime: '2026-03-29T10:00:00.000Z',
    });
  });

  it('reads paged announcement archive payload', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              ok: true,
              data: {
                items: [
                  {
                    id: 'announcement-1',
                    title: 'Current',
                    content: 'Body',
                    status: 'PUBLISHED',
                    publishTime: '2026-03-29T10:00:00.000Z',
                    expireTime: null,
                  },
                ],
                pagination: {
                  page: 2,
                  pageSize: 20,
                  totalItems: 21,
                  totalPages: 2,
                },
              },
            }),
            {
              status: 200,
              headers: {
                'content-type': 'application/json',
              },
            },
          ),
      ),
    );

    await expect(readAnnouncementArchive(2, 20)).resolves.toEqual({
      items: [
        {
          id: 'announcement-1',
          title: 'Current',
          content: 'Body',
          status: 'PUBLISHED',
          publishTime: '2026-03-29T10:00:00.000Z',
          expireTime: null,
        },
      ],
      pagination: {
        page: 2,
        pageSize: 20,
        totalItems: 21,
        totalPages: 2,
      },
    });
  });

  it('saves announcements from JSON submissions', async () => {
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        new Response(
          JSON.stringify({
            ok: true,
            data: {
              id: 'announcement-1',
            },
          }),
          {
            status: 200,
            headers: {
              'content-type': 'application/json',
            },
          },
        ),
    );
    vi.stubGlobal('fetch', fetchMock);

    const redirect = vi.fn(
      (path: string, status = 302) => new Response(null, { status, headers: { location: path } }),
    );
    const response = await saveAnnouncementPost({
      request: new Request(`${getWebBaseUrl()}/management/announcements/save`, {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          cookie: 'zhblogs_session=session-1',
        },
        body: JSON.stringify({
          title: '公告标题',
          content: '公告正文',
          submit_intent: 'publish',
          schedule_enabled: false,
          auto_expire: false,
          page: '2',
          pageSize: '20',
        }),
      }),
      redirect,
    } as unknown as Parameters<typeof saveAnnouncementPost>[0]);

    expect(fetchMock).toHaveBeenCalledWith(
      `${getApiBaseUrl()}/api/management/announcements`,
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({
          accept: 'application/json',
          'content-type': 'application/json',
          cookie: 'zhblogs_session=session-1',
        }),
      }),
    );
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(JSON.parse(String(init.body))).toEqual({
      title: '公告标题',
      content: '公告正文',
      status: 'PUBLISHED',
      publish_time: null,
      expire_time: null,
    });
    expect(response.status).toBe(302);
    expect(response.headers.get('location')).toBe(
      '/management/announcements?status=saved&page=2&pageSize=20',
    );
  });
});

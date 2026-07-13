import type { APIRoute } from 'astro';

import { getApiBaseUrl } from '@/application/auth/auth.server';

export const prerender = false;

type AnnouncementSavePayload = {
  id?: unknown;
  page?: unknown;
  pageSize?: unknown;
  submit_intent?: unknown;
  schedule_enabled?: unknown;
  auto_expire?: unknown;
  title?: unknown;
  content?: unknown;
  publish_time?: unknown;
  expire_time?: unknown;
};

const buildRedirectPath = (params: Record<string, string | null | undefined>): string => {
  const target = new URL('/management/announcements', 'http://zhblogs.local');

  for (const [key, value] of Object.entries(params)) {
    if (value) {
      target.searchParams.set(key, value);
    }
  }

  return `${target.pathname}${target.search}`;
};

const readErrorMessage = async (response: Response): Promise<string> => {
  try {
    const payload = (await response.json()) as {
      error?: {
        message?: string;
      };
    };

    if (payload.error?.message?.trim()) {
      return payload.error.message.trim();
    }
  } catch {
    // Ignore non-JSON error bodies and fall back to a generic message.
  }

  return '公告保存失败，请稍后再试。';
};

const readString = (value: unknown): string | null =>
  typeof value === 'string' && value.trim() ? value.trim() : null;

const readBoolean = (value: unknown): boolean =>
  value === true || value === 'true' || value === 'on' || value === '1';

const readSavePayload = async (request: Request): Promise<AnnouncementSavePayload> => {
  const contentType = request.headers.get('content-type') ?? '';

  if (contentType.includes('application/json')) {
    const payload = await request.json().catch(() => null);
    return payload && typeof payload === 'object' && !Array.isArray(payload) ? payload : {};
  }

  const formData = await request.formData();

  return {
    id: formData.get('id'),
    page: formData.get('page'),
    pageSize: formData.get('pageSize'),
    submit_intent: formData.get('submit_intent'),
    schedule_enabled: formData.has('schedule_enabled'),
    auto_expire: formData.has('auto_expire'),
    title: formData.get('title'),
    content: formData.get('content'),
    publish_time: formData.get('publish_time'),
    expire_time: formData.get('expire_time'),
  };
};

export const GET: APIRoute = async ({ redirect }) => redirect('/management/announcements', 302);

export const POST: APIRoute = async ({ request, redirect }) => {
  const payload = await readSavePayload(request);
  const announcementId = readString(payload.id);
  const page = readString(payload.page);
  const pageSize = readString(payload.pageSize);
  const submitIntent = readString(payload.submit_intent) ?? 'draft';
  const scheduleEnabled = readBoolean(payload.schedule_enabled);
  const autoExpireEnabled = readBoolean(payload.auto_expire);
  const status =
    submitIntent === 'publish' ? (scheduleEnabled ? 'SCHEDULED' : 'PUBLISHED') : 'DRAFT';
  const body = {
    id: announcementId ?? undefined,
    title: readString(payload.title) ?? '',
    content: readString(payload.content),
    status,
    publish_time: scheduleEnabled ? readString(payload.publish_time) : null,
    expire_time: autoExpireEnabled ? readString(payload.expire_time) : null,
  };

  try {
    const response = await fetch(`${getApiBaseUrl()}/api/management/announcements`, {
      method: 'POST',
      headers: {
        accept: 'application/json',
        'content-type': 'application/json',
        cookie: request.headers.get('cookie') ?? '',
      },
      body: JSON.stringify(body),
    });

    if (response.status === 401) {
      return redirect('/login?next=%2Fmanagement%2Fannouncements', 302);
    }

    if (response.status === 403) {
      return redirect('/forbidden', 302);
    }

    if (!response.ok) {
      const message = await readErrorMessage(response);
      return redirect(
        buildRedirectPath({
          error: 'save_failed',
          message,
          edit: announcementId,
          mode: announcementId ? undefined : 'create',
          page,
          pageSize,
        }),
        302,
      );
    }

    return redirect(
      buildRedirectPath({
        status: 'saved',
        page,
        pageSize,
      }),
      302,
    );
  } catch {
    return redirect(
      buildRedirectPath({
        error: 'save_failed',
        message: '公告保存失败，请稍后再试。',
        edit: announcementId,
        mode: announcementId ? undefined : 'create',
        page,
        pageSize,
      }),
      302,
    );
  }
};

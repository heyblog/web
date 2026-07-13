import type { APIRoute } from 'astro';

import { getApiBaseUrl } from '@/application/auth/auth.server';

export const prerender = false;

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

  return '公告归档失败，请稍后再试。';
};

const readString = (value: unknown): string | null =>
  typeof value === 'string' && value.trim() ? value.trim() : null;

const readRedirectPayload = async (
  request: Request,
): Promise<{ page: string | null; pageSize: string | null }> => {
  const contentType = request.headers.get('content-type') ?? '';

  if (contentType.includes('application/json')) {
    const payload = await request.json().catch(() => null);
    const record = payload && typeof payload === 'object' && !Array.isArray(payload) ? payload : {};
    return {
      page: readString((record as { page?: unknown }).page),
      pageSize: readString((record as { pageSize?: unknown }).pageSize),
    };
  }

  const formData = await request.formData();

  return {
    page: readString(formData.get('page')),
    pageSize: readString(formData.get('pageSize')),
  };
};

export const POST: APIRoute = async ({ request, params, redirect }) => {
  const id = params.id?.trim();

  if (!id) {
    return new Response('Invalid announcement archive request', { status: 400 });
  }

  const { page, pageSize } = await readRedirectPayload(request);

  const response = await fetch(`${getApiBaseUrl()}/api/management/announcements/${id}/archive`, {
    method: 'POST',
    headers: {
      accept: 'application/json',
      cookie: request.headers.get('cookie') ?? '',
    },
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
        error: 'archive_failed',
        message,
        page,
        pageSize,
      }),
      302,
    );
  }

  return redirect(
    buildRedirectPath({
      status: 'archived',
      page,
      pageSize,
    }),
    302,
  );
};

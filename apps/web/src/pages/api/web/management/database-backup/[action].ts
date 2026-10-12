import type { APIRoute } from 'astro';

import { forwardDatabaseBackup } from '@/api/database-backup/database-backup.proxy.server';
import { isRecord } from '@/api/database-backup/database-backup.types';

export const prerender = false;
export const ALL: APIRoute = async ({ request, params }) => {
  const response = await forwardDatabaseBackup(request, params.action ?? '');
  if (
    params.action !== 'export' ||
    response.ok ||
    !request.headers.get('accept')?.includes('text/html')
  )
    return response;
  let code = 'request_failed';
  try {
    const value: unknown = await response.json();
    if (isRecord(value) && typeof value.code === 'string' && /^[a-z_]+$/u.test(value.code))
      code = value.code;
  } catch (error) {
    if (!(error instanceof Error)) throw error;
  }
  const headers = new Headers({
    'Cache-Control': 'private, no-store',
    Location: `/management/database-backup?error=${encodeURIComponent(code)}`,
  });
  for (const cookie of response.headers.getSetCookie()) headers.append('Set-Cookie', cookie);
  return new Response(null, { status: 303, headers });
};

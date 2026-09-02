import type { APIRoute } from 'astro';

import { requestAuthAPI } from '@/application/auth/auth.server';

export const POST: APIRoute = async ({ request, params }) => {
  if (request.headers.get('Sec-Fetch-Site') !== 'same-origin')
    return new Response(null, { status: 403 });
  const body: unknown = await request.json();
  if (!isRecord(body)) return new Response(null, { status: 400 });
  return requestAuthAPI(request, `/management/site-audits/${params.auditId ?? ''}/review`, {
    method: 'POST',
    body,
  });
};

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

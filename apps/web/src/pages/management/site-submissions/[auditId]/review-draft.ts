import type { APIRoute } from 'astro';

import { requestAuthAPI } from '@/application/auth/auth.server';

export const PUT: APIRoute = async ({ request, params }) =>
  forwardReviewDraft(request, params.auditId ?? '', 'PUT');

export const DELETE: APIRoute = async ({ request, params }) =>
  forwardReviewDraft(request, params.auditId ?? '', 'DELETE');

async function forwardReviewDraft(
  request: Request,
  auditID: string,
  method: 'PUT' | 'DELETE',
): Promise<Response> {
  if (request.headers.get('Sec-Fetch-Site') !== 'same-origin')
    return new Response(null, { status: 403 });
  const body: unknown = await request.json();
  if (!isRecord(body)) return new Response(null, { status: 400 });
  return requestAuthAPI(request, `/management/site-audits/${auditID}/review-draft`, {
    method,
    body,
  });
}

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

import type { APIRoute } from 'astro';

import { pageLocation, readProblemCode, requestAuthAPI } from '@/application/auth/auth.server';
import { userRoles } from '@/application/auth/auth.types';
export const prerender = false;
export const POST: APIRoute = async ({ params, request }) => {
  const form = await request.formData();
  const role = form.get('role');
  if (
    typeof role !== 'string' ||
    !userRoles.includes(role as (typeof userRoles)[number]) ||
    !params.userId
  )
    return new Response('Invalid role', { status: 422 });
  const response = await requestAuthAPI(request, `/management/users/${params.userId}/role`, {
    method: 'PATCH',
    body: { role },
  });
  const target = response.ok
    ? '/management/users?status=updated'
    : `/management/users?error=${encodeURIComponent(await readProblemCode(response))}`;
  return Response.redirect(pageLocation(request, target), 303);
};

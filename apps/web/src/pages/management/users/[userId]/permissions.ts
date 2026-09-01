import type { APIRoute } from 'astro';

import { pageLocation, readProblemCode, requestAuthAPI } from '@/application/auth/auth.server';
import { managementPermissions } from '@/application/auth/auth.types';
export const prerender = false;
export const POST: APIRoute = async ({ params, request }) => {
  if (!params.userId) return new Response('Invalid user', { status: 422 });
  const form = await request.formData();
  const values = form.getAll('permissions');
  const permissions = values.filter(
    (value): value is (typeof managementPermissions)[number] =>
      typeof value === 'string' &&
      managementPermissions.includes(value as (typeof managementPermissions)[number]),
  );
  if (permissions.length !== values.length)
    return new Response('Invalid permissions', { status: 422 });
  const response = await requestAuthAPI(request, `/management/users/${params.userId}/permissions`, {
    method: 'PATCH',
    body: { permissions },
  });
  const target = response.ok
    ? '/management/users?status=updated'
    : `/management/users?error=${encodeURIComponent(await readProblemCode(response))}`;
  return Response.redirect(pageLocation(request, target), 303);
};

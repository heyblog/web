import type { APIRoute } from 'astro';

import { submitAuthorization } from '../../../../application/management/users-mutation.server.ts';
export const prerender = false;
export const POST: APIRoute = ({ params, request }) =>
  submitAuthorization(request, { id: params.userId, operation: 'permissions' });

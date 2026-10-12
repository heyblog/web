import type { APIRoute } from 'astro';

import { forwardManagedUserMutation } from '@/api/managed-users/managed-users.proxy.server';
export const prerender = false;
export const POST: APIRoute = ({ request, params }) =>
  forwardManagedUserMutation(request, 'password-reset', params.userId);

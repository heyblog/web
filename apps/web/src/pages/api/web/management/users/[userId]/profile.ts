import type { APIRoute } from 'astro';

import { forwardManagedUserMutation } from '@/api/managed-users/managed-users.proxy.server';
export const prerender = false;
export const PATCH: APIRoute = ({ request, params }) =>
  forwardManagedUserMutation(request, 'profile', params.userId);

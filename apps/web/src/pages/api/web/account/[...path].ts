import type { APIRoute } from 'astro';

import { forwardProfile } from '@/api/account-profile/profile.server.ts';
import { forwardSiteManagement } from '@/api/site-management/proxy.server.ts';

export const prerender = false;
export const ALL: APIRoute = ({ request, params }) => {
  const path = params.path ?? '';
  return ['profile', 'profile/confirm', 'username'].includes(path)
    ? forwardProfile(request, path)
    : forwardSiteManagement(request, path);
};

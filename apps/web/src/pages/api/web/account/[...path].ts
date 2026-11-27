import type { APIRoute } from 'astro';

import { acceptsProfilePath, forwardProfile } from '@/api/account-profile/profile.server.ts';
import { forwardSiteManagement } from '@/api/site-management/proxy.server.ts';

export const prerender = false;
export const ALL: APIRoute = ({ request, params }) => {
  const path = params.path ?? '';
  return acceptsProfilePath(path)
    ? forwardProfile(request, path)
    : forwardSiteManagement(request, path);
};

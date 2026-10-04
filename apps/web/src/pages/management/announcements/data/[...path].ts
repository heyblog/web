import type { APIRoute } from 'astro';

import { forwardAnnouncementManagement } from '../../../../api/announcements/announcements.proxy.server.ts';

export const prerender = false;
const forward: APIRoute = ({ request, params }) =>
  forwardAnnouncementManagement(request, params.path ?? '');
export const GET = forward;
export const PUT = forward;
export const POST = forward;
export const DELETE = forward;

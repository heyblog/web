import type { APIRoute } from 'astro';

import { forwardAnnouncementManagement } from '../../../api/announcements/announcements.proxy.server.ts';

export const prerender = false;
export const GET: APIRoute = ({ request }) => forwardAnnouncementManagement(request);
export const POST: APIRoute = ({ request }) => forwardAnnouncementManagement(request);

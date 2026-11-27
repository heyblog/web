import type { APIRoute } from 'astro';

import { forwardTask } from '@/api/task-center/task-center.proxy.server';
export const prerender = false;
const handler: APIRoute = ({ request, params }) =>
  forwardTask(request, 'public-signing', params.path ?? '');
export const GET = handler;
export const POST = handler;
export const PUT = handler;

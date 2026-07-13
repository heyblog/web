import type { APIRoute } from 'astro';

import { handleTaskManagementApiProxyRequest } from '@/application/management/task-management.server-handler';

export const prerender = false;

const handleRequest: APIRoute = async ({ params, request }) =>
  handleTaskManagementApiProxyRequest(params.path?.trim() ?? '', request);

export const GET = handleRequest;
export const POST = handleRequest;
export const PUT = handleRequest;
export const DELETE = handleRequest;

import type { APIRoute } from 'astro';

import { forwardDatabaseBackup } from '@/api/database-backup/database-backup.proxy.server';

export const prerender = false;
export const ALL: APIRoute = ({ request, params }) =>
  forwardDatabaseBackup(request, params.upload ? `uploads/${params.upload}` : 'uploads');

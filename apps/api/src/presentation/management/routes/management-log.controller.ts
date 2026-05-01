import { readFile } from 'node:fs/promises';

import type { FastifyInstance } from 'fastify';

import { resolveApiLogFilePath } from '@/shared/runtime/service/app-logger.service';

import { requireManagementPermission } from './management-route.shared';

export function registerManagementLogRoutes(app: FastifyInstance): void {
  app.get(
    '/api/management/logs',
    {
      preHandler: requireManagementPermission('log.read'),
    },
    async (request) => {
      const query = request.query as { tail?: string; q?: string };
      const tail = Math.min(Math.max(Number(query.tail ?? 200) || 200, 10), 500);
      const keyword = query.q?.trim().toLowerCase() ?? '';
      const env = app.config.NODE_ENV;
      const filePath = resolveApiLogFilePath(app.config);

      const raw = await readFile(filePath, 'utf8').catch(() => '');
      const lines = raw
        .split('\n')
        .filter(Boolean)
        .filter((line) => (keyword ? line.toLowerCase().includes(keyword) : true))
        .slice(-tail);

      return {
        ok: true,
        data: {
          env,
          file_path: filePath,
          lines,
        },
      };
    },
  );
}

import { requestAuthAPI } from '../auth/auth.server.ts';

import { type CurrentBackupUpload, parseCurrentBackupUpload } from './database-backup.uploads.ts';

export async function readBackupUploads(
  request: Request,
): Promise<{ readonly current: CurrentBackupUpload | null; readonly cookies: readonly string[] }> {
  const response = await requestAuthAPI(
    request,
    '/api/web/management/database-backup/uploads/current',
  );
  const cookies = response.headers.getSetCookie();
  if (!response.ok) return { current: null, cookies };
  try {
    const value: unknown = await response.json();
    return { current: parseCurrentBackupUpload(value), cookies };
  } catch (error) {
    if (error instanceof Error) return { current: null, cookies };
    throw error;
  }
}

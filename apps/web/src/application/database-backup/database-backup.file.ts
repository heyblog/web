import { backupFileLimit } from '../../api/database-backup/database-backup.types.ts';

export function backupFileIssue(
  file: Pick<File, 'size'> | null,
): 'empty_file' | 'request_too_large' | null {
  if (file?.size === 0) return 'empty_file';
  if (file && file.size > backupFileLimit) return 'request_too_large';
  return null;
}

import {
  backupDatasetNames,
  type BackupRestoration,
  isRecord,
  parseBackupRestoration,
} from './database-backup.types.ts';

export interface RestoreProgress {
  readonly stage: 'validating' | 'restoring' | 'checking' | 'committing' | 'rolling_back';
  readonly dataset: string;
  readonly completed: number;
  readonly total: number;
}
interface JobBase {
  readonly id: string;
  readonly progress: RestoreProgress;
}
export type RestoreJob = JobBase &
  (
    | {
        readonly status: 'running' | 'cancelling';
        readonly result?: never;
        readonly error_code?: never;
      }
    | {
        readonly status: 'succeeded';
        readonly result: BackupRestoration;
        readonly error_code?: never;
      }
    | {
        readonly status: 'failed' | 'cancelled' | 'unknown';
        readonly result?: never;
        readonly error_code: string;
      }
  );
export function isActiveJob(job: RestoreJob | null): boolean {
  return job?.status === 'running' || job?.status === 'cancelling';
}
export function parseRestoreJob(value: unknown): RestoreJob | null {
  if (
    !isRecord(value) ||
    typeof value.id !== 'string' ||
    !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/iu.test(value.id) ||
    !isRecord(value.progress)
  )
    return null;
  const p = value.progress;
  if (
    p.stage !== 'validating' &&
    p.stage !== 'restoring' &&
    p.stage !== 'checking' &&
    p.stage !== 'committing' &&
    p.stage !== 'rolling_back'
  )
    return null;
  if (
    typeof p.dataset !== 'string' ||
    (p.dataset !== '' &&
      !backupDatasetNames.some((name) => name === p.dataset) &&
      p.dataset !== 'graph.vertices' &&
      p.dataset !== 'graph.edges') ||
    typeof p.completed !== 'number' ||
    !Number.isSafeInteger(p.completed) ||
    p.completed < 0 ||
    typeof p.total !== 'number' ||
    !Number.isSafeInteger(p.total) ||
    p.total < p.completed
  )
    return null;
  const base = {
    id: value.id,
    progress: { stage: p.stage, dataset: p.dataset, completed: p.completed, total: p.total },
  } satisfies JobBase;
  if (value.status === 'succeeded') {
    const result = parseBackupRestoration(value.result);
    return result && value.error_code === undefined
      ? { ...base, status: 'succeeded', result }
      : null;
  }
  if (value.result !== undefined) return null;
  if (value.status === 'running' || value.status === 'cancelling')
    return value.error_code === undefined ? { ...base, status: value.status } : null;
  if (value.status === 'failed' || value.status === 'cancelled' || value.status === 'unknown')
    return typeof value.error_code === 'string' && /^[a-z_]+$/u.test(value.error_code)
      ? { ...base, status: value.status, error_code: value.error_code }
      : null;
  return null;
}
export function parseCurrentRestoreJob(value: unknown): { readonly job: RestoreJob | null } | null {
  if (!isRecord(value)) return null;
  if (value.job === null) return { job: null };
  const job = parseRestoreJob(value.job);
  return job ? { job } : null;
}

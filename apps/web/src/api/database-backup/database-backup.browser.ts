import { parseCurrentRestoreJob, parseRestoreJob } from './database-backup.jobs.ts';
import { backupTimeoutMs, isRecord, parseBackupInspection } from './database-backup.types.ts';

export class BackupTransportError extends Error {
  readonly name = 'BackupTransportError';
  readonly code: string;
  readonly status: number;
  constructor(code: string, status = 0) {
    super(code);
    this.code = code;
    this.status = status;
  }
}

export function createBackupTransport(fetcher: typeof fetch = fetch) {
  async function upload(
    action: string,
    form: FormData | undefined,
    signal: AbortSignal,
  ): Promise<Response> {
    let response: Response;
    const operationSignal = AbortSignal.any([signal, AbortSignal.timeout(backupTimeoutMs)]);
    try {
      response = await fetcher(`/api/web/management/database-backup/${action}`, {
        method: form || action.endsWith('/cancel') ? 'POST' : 'GET',
        body: form,
        credentials: 'same-origin',
        cache: 'no-store',
        headers: { Accept: 'application/json' },
        signal: operationSignal,
      });
    } catch (error) {
      if (error instanceof Error) {
        if (signal.aborted) throw new BackupTransportError('cancelled');
        throw new BackupTransportError(
          action === 'restore'
            ? 'restore_outcome_unknown'
            : operationSignal.aborted
              ? 'request_timeout'
              : 'network_error',
        );
      }
      throw error;
    }
    if (!response.ok) {
      const timedOut = [408, 504, 524].includes(response.status);
      let code = timedOut ? 'request_timeout' : 'request_failed';
      try {
        const problem: unknown = await response.json();
        if (isRecord(problem) && typeof problem.code === 'string') code = problem.code;
      } catch (error) {
        if (!(error instanceof Error)) throw error;
      }
      throw new BackupTransportError(
        action === 'restore' && (timedOut || response.status >= 500)
          ? 'restore_outcome_unknown'
          : code,
        response.status,
      );
    }
    return response;
  }
  async function decode<T>(response: Response, parser: (value: unknown) => T | null): Promise<T> {
    let value: unknown;
    try {
      value = await response.json();
    } catch (error) {
      if (error instanceof Error) throw new BackupTransportError('invalid_response');
      throw error;
    }
    const result = parser(value);
    if (result === null) throw new BackupTransportError('invalid_response');
    return result;
  }
  return {
    async current(signal: AbortSignal) {
      return decode(
        await upload('restore-jobs/current', undefined, signal),
        parseCurrentRestoreJob,
      );
    },
    async job(id: string, signal: AbortSignal) {
      return decode(
        await upload(`restore-jobs/${encodeURIComponent(id)}`, undefined, signal),
        parseRestoreJob,
      );
    },
    async cancel(id: string, signal: AbortSignal) {
      return decode(
        await upload(`restore-jobs/${encodeURIComponent(id)}/cancel`, undefined, signal),
        parseRestoreJob,
      );
    },
    async inspect(file: File, signal: AbortSignal) {
      const form = new FormData();
      form.set('file', file);
      return decode(await upload('inspect', form, signal), parseBackupInspection);
    },
    async restore(file: File, sha256: string, signal: AbortSignal) {
      const form = new FormData();
      form.set('file', file);
      form.set('sha256', sha256);
      form.set('confirmation', 'RESTORE');
      let result;
      try {
        result = await decode(await upload('restore', form, signal), parseRestoreJob);
      } catch (error) {
        if (error instanceof BackupTransportError && error.code === 'invalid_response')
          throw new BackupTransportError('restore_outcome_unknown');
        throw error;
      }
      if (result.status === 'succeeded' && result.result.sha256 !== sha256)
        throw new BackupTransportError('restore_outcome_unknown');
      return result;
    },
  };
}

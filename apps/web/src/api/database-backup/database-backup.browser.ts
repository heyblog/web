import { parseCurrentRestoreJob, parseRestoreJob } from './database-backup.jobs.ts';
import { backupTimeoutMs, isRecord, parseBackupInspection } from './database-backup.types.ts';
import {
  parseBackupUpload,
  parseCurrentBackupUpload,
  parsePartURL,
} from './database-backup.uploads.ts';

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
    body: object | undefined,
    signal: AbortSignal,
    method = body || action.endsWith('/cancel') ? 'POST' : 'GET',
  ): Promise<Response> {
    let response: Response;
    const operationSignal = AbortSignal.any([signal, AbortSignal.timeout(backupTimeoutMs)]);
    try {
      response = await fetcher(`/api/web/management/database-backup/${action}`, {
        method,
        body: body === undefined ? undefined : JSON.stringify(body),
        credentials: 'same-origin',
        cache: 'no-store',
        headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
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
    async uploads(signal: AbortSignal) {
      return decode(await upload('uploads/current', undefined, signal), parseCurrentBackupUpload);
    },
    async upload(id: string, signal: AbortSignal) {
      return decode(
        await upload(`uploads/${encodeURIComponent(id)}`, undefined, signal),
        parseBackupUpload,
      );
    },
    async createUpload(size: number, part_sha256: readonly string[], signal: AbortSignal) {
      return decode(await upload('uploads', { size, part_sha256 }, signal), parseBackupUpload);
    },
    async part(id: string, number: number, signal: AbortSignal) {
      return decode(
        await upload(`uploads/${encodeURIComponent(id)}/parts/${number}`, {}, signal),
        parsePartURL,
      );
    },
    async complete(id: string, signal: AbortSignal) {
      return decode(
        await upload(`uploads/${encodeURIComponent(id)}/complete`, {}, signal),
        parseBackupUpload,
      );
    },
    async discard(id: string, signal: AbortSignal) {
      return decode(
        await upload(`uploads/${encodeURIComponent(id)}`, undefined, signal, 'DELETE'),
        parseBackupUpload,
      );
    },
    async inspect(id: string, signal: AbortSignal) {
      return decode(await upload('inspect', { upload_id: id }, signal), parseBackupInspection);
    },
    async restore(id: string, sha256: string, signal: AbortSignal) {
      let result;
      try {
        result = await decode(
          await upload('restore', { upload_id: id, sha256, confirmation: 'RESTORE' }, signal),
          parseRestoreJob,
        );
      } catch (error) {
        if (error instanceof BackupTransportError && error.code === 'invalid_response')
          throw new BackupTransportError('restore_outcome_unknown');
        throw error;
      }
      if (
        result.upload_id !== id ||
        (result.status === 'succeeded' && result.result.sha256 !== sha256)
      )
        throw new BackupTransportError('restore_outcome_unknown');
      return result;
    },
  };
}

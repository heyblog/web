import {
  backupTimeoutMs,
  isRecord,
  parseBackupInspection,
  parseBackupRestoration,
} from './database-backup.types.ts';

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
    action: 'inspect' | 'restore',
    form: FormData,
    signal: AbortSignal,
  ): Promise<Response> {
    let response: Response;
    try {
      response = await fetcher(`/management/database-backup/${action}`, {
        method: 'POST',
        body: form,
        credentials: 'same-origin',
        cache: 'no-store',
        headers: { Accept: 'application/json' },
        signal: AbortSignal.any([signal, AbortSignal.timeout(backupTimeoutMs)]),
      });
    } catch (error) {
      if (error instanceof Error) {
        if (signal.aborted) throw new BackupTransportError('cancelled');
        throw new BackupTransportError(
          action === 'restore' ? 'restore_outcome_unknown' : 'network_error',
        );
      }
      throw error;
    }
    if (!response.ok) {
      let code = 'request_failed';
      try {
        const problem: unknown = await response.json();
        if (isRecord(problem) && typeof problem.code === 'string') code = problem.code;
      } catch (error) {
        if (!(error instanceof Error)) throw error;
      }
      throw new BackupTransportError(code, response.status);
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
        result = await decode(await upload('restore', form, signal), parseBackupRestoration);
      } catch (error) {
        if (error instanceof BackupTransportError && error.code === 'invalid_response')
          throw new BackupTransportError('restore_outcome_unknown');
        throw error;
      }
      if (result.sha256 !== sha256) throw new BackupTransportError('restore_outcome_unknown');
      return result;
    },
  };
}

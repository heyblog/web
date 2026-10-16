import {
  BackupTransportError,
  type createBackupTransport,
} from '../../api/database-backup/database-backup.browser.ts';
import { BackupTransferMeter } from '../../api/database-backup/database-backup.progress.ts';
import { stageBackupFile } from '../../api/database-backup/database-backup.transfer.browser.ts';
import type { BackupInspection } from '../../api/database-backup/database-backup.types.ts';
import type {
  BackupUpload,
  BackupUploadSettings,
  CurrentBackupUpload,
} from '../../api/database-backup/database-backup.uploads.ts';

import { backupFileIssue } from './database-backup.file.ts';
import { backupMessage } from './database-backup.messages.ts';

export class BackupUploadController {
  file = $state<File | null>(null);
  session = $state<BackupUpload | null>(null);
  settings = $state<BackupUploadSettings | null>(null);
  inspection = $state<BackupInspection | null>(null);
  busy = $state<'hashing' | 'uploading' | 'inspecting' | 'discarding' | 'preparing' | null>(null);
  bytes = $state(0);
  speed = $state(0);
  error = $state<string | null>(null);
  private operation: AbortController | null = null;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private disposed = false;
  private readonly meter = new BackupTransferMeter();
  private readonly uploadOrigin: string | null;
  private readonly uploadTransport: BackupUploadSettings['transport'] | null;
  private readonly conflict: () => Promise<void>;
  private readonly transport: ReturnType<typeof createBackupTransport>;
  constructor(
    transport: ReturnType<typeof createBackupTransport>,
    initial: CurrentBackupUpload | null,
    conflict: () => Promise<void>,
  ) {
    this.uploadOrigin = initial?.settings.upload_origin ?? null;
    this.uploadTransport = initial?.settings.transport ?? null;
    this.conflict = conflict;
    this.transport = transport;
    this.settings = initial?.settings ?? null;
    this.session = initial?.upload ?? null;
    this.inspection = this.usable ? (initial?.upload?.inspection ?? null) : null;
    this.bytes = initial?.upload?.uploaded_bytes ?? 0;
  }
  get preparing() {
    return this.session?.state === 'preparing' || this.session?.state === 'inspecting';
  }
  get blocked() {
    return this.busy !== null || this.preparing;
  }
  get usable() {
    return this.session?.state === 'ready' || this.session?.state === 'inspected';
  }
  select(file: File | null) {
    if (this.blocked) return;
    this.file = file;
    const issue = backupFileIssue(file);
    this.error = issue ? backupMessage(issue) : null;
  }
  async refresh(signal: AbortSignal) {
    const current = await this.transport.uploads(signal);
    if (this.disposed) return;
    const settingsMatch =
      current.settings.transport === this.uploadTransport &&
      current.settings.upload_origin === this.uploadOrigin;
    this.settings = settingsMatch
      ? current.settings
      : { ...current.settings, transport: 'disabled', enabled: false, upload_origin: '' };
    if (!settingsMatch) this.error = backupMessage('upload_settings_changed');
    this.accept(current.upload);
  }
  private accept(session: BackupUpload | null) {
    if (this.disposed) return;
    if (session?.state === 'preparing') this.speed = this.meter.record(session.prepared_bytes);
    else this.meter.reset();
    if (session?.state !== this.session?.state && !session?.error_code && this.settings?.enabled)
      this.error = null;
    this.session = session;
    this.bytes = session?.uploaded_bytes ?? 0;
    this.inspection =
      session?.state === 'ready' || session?.state === 'inspected'
        ? (session.inspection ?? null)
        : null;
    if (session?.error_code) this.error = backupMessage(session.error_code);
    this.schedule();
  }
  async start() {
    if (
      !this.file ||
      this.blocked ||
      !this.settings?.enabled ||
      backupFileIssue(this.file) !== null
    )
      return;
    const operation = new AbortController();
    this.operation = operation;
    this.busy = 'hashing';
    this.error = null;
    try {
      let lastUpdate = 0;
      this.accept(
        await stageBackupFile({
          session: this.session,
          file: this.file,
          settings: this.settings,
          transport: this.transport,
          signal: operation.signal,
          uploading: (session) => {
            this.busy = 'uploading';
            this.accept(session);
          },
          progress: (progress) => {
            if (performance.now() - lastUpdate < 250 && progress.bytes !== this.file?.size) return;
            lastUpdate = performance.now();
            this.bytes = progress.bytes;
            this.speed = progress.bytesPerSecond;
          },
        }),
      );
    } catch (error) {
      this.report(error, operation.signal);
      if (
        error instanceof BackupTransportError &&
        ['network_error', 'request_timeout', 'invalid_response', 'database_backup_busy'].includes(
          error.code,
        ) &&
        !operation.signal.aborted
      ) {
        try {
          await this.refresh(operation.signal);
        } catch (recoveryError) {
          this.report(recoveryError, operation.signal);
        }
      }
    } finally {
      this.busy = null;
      this.operation = null;
      this.speed = 0;
      this.schedule();
    }
  }
  pause() {
    if (this.busy === 'hashing' || this.busy === 'uploading') this.operation?.abort();
  }
  async prepare() {
    if (!this.session || this.blocked || this.session.state !== 'preparation_failed') return;
    this.operation = new AbortController();
    this.busy = 'preparing';
    this.error = null;
    try {
      this.accept(await this.transport.complete(this.session.id, this.operation.signal));
    } catch (error) {
      this.report(error, this.operation.signal);
    } finally {
      this.operation = null;
      this.busy = null;
      this.schedule();
    }
  }
  async inspect() {
    if (!this.session || !this.usable || this.blocked) return;
    const operation = new AbortController();
    this.operation = operation;
    this.busy = 'inspecting';
    this.error = null;
    try {
      this.inspection = await this.transport.inspect(this.session.id, operation.signal);
      this.session = {
        ...this.session,
        state: 'inspected',
        inspection: this.inspection,
        sha256: this.inspection.sha256,
      };
    } catch (error) {
      this.report(error, operation.signal);
      if (
        error instanceof BackupTransportError &&
        error.code === 'database_backup_busy' &&
        !operation.signal.aborted
      )
        await this.conflict();
    } finally {
      this.busy = null;
      this.operation = null;
    }
  }
  async discard() {
    if (!this.session || this.busy !== null || this.session.state === 'restoring') return;
    clearTimeout(this.timer);
    this.operation?.abort();
    this.operation = new AbortController();
    this.busy = 'discarding';
    this.error = null;
    try {
      this.accept(await this.transport.discard(this.session.id, this.operation.signal));
      this.file = null;
    } catch (error) {
      this.report(error, this.operation.signal);
    } finally {
      this.busy = null;
      this.operation = null;
      this.schedule();
    }
  }
  private schedule() {
    clearTimeout(this.timer);
    if (this.disposed || !this.session) return;
    if (this.session.state === 'ready' && !this.blocked && !this.error) {
      this.timer = setTimeout(() => {
        void this.inspect();
      }, 0);
    } else if (
      this.preparing ||
      this.session.cleanup_status === 'pending' ||
      this.session.state === 'restoring'
    ) {
      this.timer = setTimeout(() => {
        void this.update();
      }, 1000);
    }
  }
  private async update() {
    if (!this.session || this.disposed || this.operation) {
      this.schedule();
      return;
    }
    const operation = new AbortController();
    this.operation = operation;
    try {
      this.accept(await this.transport.upload(this.session.id, operation.signal));
    } catch (error) {
      this.report(error, operation.signal);
    } finally {
      if (this.operation === operation) this.operation = null;
      this.schedule();
    }
  }
  private report(error: unknown, signal: AbortSignal) {
    if (!(error instanceof BackupTransportError)) throw error;
    if (!this.disposed && !signal.aborted) this.error = backupMessage(error.code);
  }
  dispose() {
    this.disposed = true;
    clearTimeout(this.timer);
    this.operation?.abort();
  }
}

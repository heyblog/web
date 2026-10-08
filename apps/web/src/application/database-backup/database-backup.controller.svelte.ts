import {
  BackupTransportError,
  createBackupTransport,
} from '../../api/database-backup/database-backup.browser.ts';
import {
  backupFileLimit,
  type BackupInspection,
  type BackupRestoration,
} from '../../api/database-backup/database-backup.types.ts';

import { backupMessage } from './database-backup.messages.ts';

export function createBackupController(transport = createBackupTransport()) {
  return new BackupController(transport);
}
class BackupController {
  file = $state<File | null>(null);
  inspection = $state<BackupInspection | null>(null);
  restoration = $state<BackupRestoration | null>(null);
  busy = $state<'inspect' | 'restore' | null>(null);
  error = $state<string | null>(null);
  private operation: AbortController | null = null;
  private readonly transport: ReturnType<typeof createBackupTransport>;
  constructor(transport: ReturnType<typeof createBackupTransport>) {
    this.transport = transport;
  }
  select(file: File | null) {
    if (this.busy !== null) return;
    this.file = file;
    this.inspection = null;
    this.restoration = null;
    this.error = null;
    if (file?.size === 0) this.error = backupMessage('empty_file');
    if (file && file.size > backupFileLimit) this.error = backupMessage('request_too_large');
  }
  async inspect() {
    if (
      !this.file ||
      this.busy !== null ||
      this.file.size === 0 ||
      this.file.size > backupFileLimit
    )
      return;
    this.busy = 'inspect';
    this.error = null;
    this.inspection = null;
    this.restoration = null;
    this.operation = new AbortController();
    try {
      this.inspection = await this.transport.inspect(this.file, this.operation.signal);
    } catch (error) {
      if (error instanceof BackupTransportError) this.error = backupMessage(error.code);
      else throw error;
    } finally {
      this.busy = null;
      this.operation = null;
    }
  }
  async restore() {
    if (!this.file || !this.inspection?.can_restore || this.busy !== null) return;
    const inspection = this.inspection;
    this.busy = 'restore';
    this.error = null;
    this.restoration = null;
    this.operation = new AbortController();
    try {
      const result = await this.transport.restore(
        this.file,
        inspection.sha256,
        this.operation.signal,
      );
      if (
        result.admin_mapping.source_id !== inspection.excluded_system_admin_id ||
        result.admin_mapping.target_id !== inspection.retained_system_admin_id
      ) {
        this.error = backupMessage('restore_outcome_unknown');
      } else this.restoration = result;
    } catch (error) {
      if (error instanceof BackupTransportError) this.error = backupMessage(error.code);
      else throw error;
    } finally {
      this.inspection = null;
      this.busy = null;
      this.operation = null;
    }
  }
  cancelInspection() {
    if (this.busy === 'inspect') this.operation?.abort();
  }
  dispose() {
    this.operation?.abort();
  }
}

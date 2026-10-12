import {
  BackupTransportError,
  createBackupTransport,
} from '../../api/database-backup/database-backup.browser.ts';
import { isActiveJob, type RestoreJob } from '../../api/database-backup/database-backup.jobs.ts';
import {
  backupFileLimit,
  type BackupInspection,
} from '../../api/database-backup/database-backup.types.ts';

import { backupMessage } from './database-backup.messages.ts';

export function createBackupController(transport = createBackupTransport()) {
  return new BackupController(transport);
}
class BackupController {
  file = $state<File | null>(null);
  inspection = $state<BackupInspection | null>(null);
  job = $state<RestoreJob | null>(null);
  busy = $state<'inspect' | 'restore' | 'cancel' | null>(null);
  error = $state<string | null>(null);
  checking = $state(true);
  acceptanceUnknown = $state(false);
  private operation: AbortController | null = null;
  private poll: AbortController | null = null;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private disposed = false;
  private uncertainUpload = false;
  private latestJobID: string | null = null;
  private reviewed: BackupInspection | null = null;
  private readonly transport: ReturnType<typeof createBackupTransport>;
  constructor(transport: ReturnType<typeof createBackupTransport>) {
    this.transport = transport;
  }
  get active() {
    return isActiveJob(this.job);
  }
  get blocked() {
    return this.busy !== null || this.active || this.checking || this.acceptanceUnknown;
  }
  get restoration() {
    return this.job?.status === 'succeeded' ? this.job.result : null;
  }
  select(file: File | null) {
    if (this.blocked) return;
    this.file = file;
    this.inspection = null;
    this.job = null;
    this.error = null;
    if (file?.size === 0) this.error = backupMessage('empty_file');
    if (file && file.size > backupFileLimit) this.error = backupMessage('request_too_large');
  }
  async inspect() {
    if (!this.file || this.blocked || this.file.size === 0 || this.file.size > backupFileLimit)
      return;
    this.busy = 'inspect';
    this.error = null;
    this.inspection = null;
    this.job = null;
    this.operation = new AbortController();
    try {
      this.inspection = await this.transport.inspect(this.file, this.operation.signal);
    } catch (error) {
      this.report(error);
    } finally {
      this.busy = null;
      this.operation = null;
    }
  }
  async restore() {
    if (!this.file || !this.inspection?.can_restore || this.blocked) return;
    this.reviewed = this.inspection;
    this.busy = 'restore';
    this.error = null;
    this.job = null;
    this.operation = new AbortController();
    try {
      this.accept(
        await this.transport.restore(this.file, this.inspection.sha256, this.operation.signal),
      );
    } catch (error) {
      this.report(error);
      if (
        !this.disposed &&
        error instanceof BackupTransportError &&
        (error.code === 'restore_outcome_unknown' || error.code === 'database_backup_busy')
      ) {
        this.uncertainUpload = error.code === 'restore_outcome_unknown';
        this.acceptanceUnknown = true;
        await this.refresh();
      }
    } finally {
      this.inspection = null;
      this.busy = null;
      this.operation = null;
    }
  }
  async refresh() {
    if (this.disposed || (this.checking && this.poll)) return;
    clearTimeout(this.timer);
    this.poll?.abort();
    this.poll = new AbortController();
    this.checking = true;
    try {
      const result = await this.transport.current(this.poll.signal);
      if (this.disposed) return;
      if (this.uncertainUpload && (!result.job || result.job.id === this.latestJobID)) {
        this.error = backupMessage('restore_outcome_unknown');
        return;
      }
      this.uncertainUpload = false;
      this.acceptanceUnknown = false;
      this.error = null;
      if (result.job) this.accept(result.job);
      else this.job = null;
    } catch (error) {
      this.report(error);
      this.acceptanceUnknown = true;
    } finally {
      this.checking = false;
      this.poll = null;
    }
  }
  async cancel() {
    if (this.job?.status !== 'running' || this.busy !== null) return;
    this.busy = 'cancel';
    this.error = null;
    clearTimeout(this.timer);
    this.poll?.abort();
    this.operation = new AbortController();
    try {
      this.accept(await this.transport.cancel(this.job.id, this.operation.signal));
    } catch (error) {
      this.report(error);
      this.schedule();
    } finally {
      this.busy = null;
      this.operation = null;
    }
  }
  private accept(job: RestoreJob) {
    if (this.disposed) return;
    if (
      job.status === 'succeeded' &&
      this.reviewed &&
      (job.result.sha256 !== this.reviewed.sha256 ||
        job.result.admin_mapping.source_id !== this.reviewed.excluded_system_admin_id ||
        job.result.admin_mapping.target_id !== this.reviewed.retained_system_admin_id)
    ) {
      this.job = null;
      this.error = backupMessage('restore_outcome_unknown');
      this.acceptanceUnknown = true;
      return;
    }
    this.job = job;
    this.latestJobID = job.id;
    if (job.status === 'unknown') this.error = backupMessage('restore_outcome_unknown');
    else if (job.status === 'failed') this.error = backupMessage(job.error_code);
    this.schedule();
  }
  private schedule() {
    clearTimeout(this.timer);
    if (this.active && !this.disposed)
      this.timer = setTimeout(() => {
        void this.update();
      }, 1000);
  }
  private async update() {
    if (!this.job || this.disposed) return;
    const poll = new AbortController();
    this.poll = poll;
    try {
      const job = await this.transport.job(this.job.id, poll.signal);
      if (!poll.signal.aborted) {
        this.error = null;
        this.accept(job);
      }
    } catch (error) {
      if (!poll.signal.aborted) {
        this.report(error);
        this.schedule();
      }
    } finally {
      if (this.poll === poll) this.poll = null;
    }
  }
  private report(error: unknown) {
    if (error instanceof BackupTransportError) {
      if (!this.disposed) this.error = backupMessage(error.code);
    } else throw error;
  }
  cancelInspection() {
    if (this.busy === 'inspect') this.operation?.abort();
  }
  dispose() {
    this.disposed = true;
    clearTimeout(this.timer);
    this.operation?.abort();
    this.poll?.abort();
  }
}

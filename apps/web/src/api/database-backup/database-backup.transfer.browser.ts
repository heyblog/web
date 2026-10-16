import { BackupTransportError, type createBackupTransport } from './database-backup.browser.ts';
import { BackupTransferMeter } from './database-backup.progress.ts';
import { isRecord } from './database-backup.types.ts';
import {
  backupPartSize,
  type BackupUpload,
  type BackupUploadSettings,
} from './database-backup.uploads.ts';

type EnabledUploadSettings = Extract<BackupUploadSettings, { readonly enabled: true }>;

export interface UploadProgress {
  readonly bytes: number;
  readonly bytesPerSecond: number;
}
export async function hashBackupFile(file: File, signal: AbortSignal): Promise<readonly string[]> {
  if (!globalThis.crypto?.subtle) throw new BackupTransportError('secure_context_required');
  const hashes: string[] = [];
  for (let offset = 0; offset < file.size; offset += backupPartSize) {
    if (signal.aborted) throw new BackupTransportError('cancelled');
    const digest = await crypto.subtle.digest(
      'SHA-256',
      await file.slice(offset, offset + backupPartSize).arrayBuffer(),
    );
    hashes.push(
      Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join(''),
    );
  }
  return hashes;
}
type PartDestination =
  | { readonly transport: 'cos'; readonly url: string; readonly origin: string }
  | { readonly transport: 'direct'; readonly id: string; readonly number: number };
type PartRequest = PartDestination & {
  readonly body: Blob;
  readonly signal: AbortSignal;
  readonly progress: (bytes: number) => void;
};
export function putBackupPart(request: PartRequest): Promise<void> {
  let url: string;
  switch (request.transport) {
    case 'cos':
      if (URL.parse(request.url)?.origin !== request.origin)
        return Promise.reject(new BackupTransportError('invalid_response'));
      url = request.url;
      break;
    case 'direct':
      url = `/api/web/management/database-backup/uploads/${encodeURIComponent(request.id)}/parts/${request.number}`;
      break;
    default:
      return assertNever(request);
  }
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const abort = () => xhr.abort();
    const finish = (error?: BackupTransportError) => {
      request.signal.removeEventListener('abort', abort);
      if (error) reject(error);
      else resolve();
    };
    xhr.open('PUT', url);
    xhr.withCredentials = request.transport === 'direct';
    xhr.timeout = 120_000;
    xhr.setRequestHeader('Content-Type', 'application/octet-stream');
    if (request.transport === 'direct') xhr.setRequestHeader('Accept', 'application/json');
    xhr.upload.onprogress = (event) => request.progress(Math.min(event.loaded, request.body.size));
    xhr.onload = () => {
      const accepted =
        request.transport === 'direct' ? xhr.status === 204 : xhr.status >= 200 && xhr.status < 300;
      if (accepted) return finish();
      let code = 'upload_failed';
      if (request.transport === 'direct' && xhr.status >= 400) {
        try {
          const value: unknown = JSON.parse(xhr.responseText);
          if (isRecord(value) && typeof value.code === 'string' && /^[a-z_]+$/u.test(value.code))
            code = value.code;
        } catch (error) {
          if (!(error instanceof SyntaxError)) throw error;
        }
      }
      finish(new BackupTransportError(code, xhr.status));
    };
    xhr.onerror = () => finish(new BackupTransportError('network_error'));
    xhr.ontimeout = () => finish(new BackupTransportError('request_timeout'));
    xhr.onabort = () => finish(new BackupTransportError('cancelled'));
    request.signal.addEventListener('abort', abort, { once: true });
    if (request.signal.aborted) finish(new BackupTransportError('cancelled'));
    else xhr.send(request.body);
  });
}
interface UploadTransfer {
  readonly session: BackupUpload;
  readonly file: File;
  readonly settings: EnabledUploadSettings;
  readonly signal: AbortSignal;
  readonly transport: ReturnType<typeof createBackupTransport>;
  readonly progress: (value: UploadProgress) => void;
}
interface BackupStaging {
  readonly session: BackupUpload | null;
  readonly file: File;
  readonly settings: EnabledUploadSettings;
  readonly signal: AbortSignal;
  readonly transport: ReturnType<typeof createBackupTransport>;
  readonly progress: (value: UploadProgress) => void;
  readonly uploading: (session: BackupUpload) => void;
}
export async function stageBackupFile(staging: BackupStaging): Promise<BackupUpload> {
  const hashes = await hashBackupFile(staging.file, staging.signal);
  let session = staging.session;
  if (session?.state === 'uploading') {
    if (
      session.size !== staging.file.size ||
      hashes.some((hash, index) => hash !== session?.part_sha256[index])
    )
      throw new BackupTransportError('different_file');
    session = await staging.transport.upload(session.id, staging.signal);
  } else {
    if (session && session.cleanup_status !== 'completed')
      throw new BackupTransportError('upload_exists');
    session = await staging.transport.createUpload(staging.file.size, hashes, staging.signal);
  }
  if (session.state !== 'uploading') throw new BackupTransportError('upload_exists');
  staging.uploading(session);
  await transferBackupFile({ ...staging, session });
  return staging.transport.complete(session.id, staging.signal);
}
export async function transferBackupFile(transfer: UploadTransfer): Promise<void> {
  const controller = new AbortController();
  const signal = AbortSignal.any([transfer.signal, controller.signal]);
  const confirmed = new Set(transfer.session.uploaded_parts);
  const inflight = new Map<number, number>();
  const meter = new BackupTransferMeter();
  let previous = 0;
  let transferred = 0;
  const partBytes = (part: number) =>
    Math.min(
      transfer.session.part_size,
      transfer.file.size - (part - 1) * transfer.session.part_size,
    );
  const report = () => {
    const bytes =
      [...confirmed].reduce((sum, part) => sum + partBytes(part), 0) +
      [...inflight.values()].reduce((sum, value) => sum + value, 0);
    transferred += Math.max(0, bytes - previous);
    previous = bytes;
    const bytesPerSecond = meter.record(transferred);
    transfer.progress({
      bytes: inflight.size > 0 && bytes === transfer.file.size ? Math.floor(bytes * 0.999) : bytes,
      bytesPerSecond,
    });
  };
  const queue = transfer.session.part_sha256
    .map((_, index) => index + 1)
    .filter((part) => !confirmed.has(part));
  async function send(part: number) {
    const body = transfer.file.slice(
      (part - 1) * transfer.session.part_size,
      part * transfer.session.part_size,
    );
    for (let attempt = 0; attempt < 3; attempt += 1) {
      try {
        const destination = await partDestination(transfer, part, signal);
        await putBackupPart({
          ...destination,
          body,
          signal,
          progress(bytes) {
            inflight.set(part, bytes);
            report();
          },
        });
        confirmed.add(part);
        inflight.delete(part);
        report();
        return;
      } catch (error) {
        inflight.delete(part);
        report();
        if (!(error instanceof BackupTransportError) || signal.aborted) throw error;
        const latest = await transfer.transport.upload(transfer.session.id, signal);
        if (latest.uploaded_parts.includes(part)) {
          confirmed.add(part);
          report();
          return;
        }
        if (attempt === 2 || (error.status >= 400 && error.status < 500 && error.status !== 403))
          throw error;
        await new Promise<void>((resolve) => {
          const timer = setTimeout(done, 1000 * (attempt + 1));
          function done() {
            clearTimeout(timer);
            signal.removeEventListener('abort', done);
            resolve();
          }
          signal.addEventListener('abort', done, { once: true });
        });
      }
    }
  }
  async function worker() {
    for (;;) {
      const part = queue.shift();
      if (part === undefined) return;
      if (signal.aborted) throw new BackupTransportError('cancelled');
      await send(part);
    }
  }
  report();
  try {
    await Promise.all([worker(), worker()]);
  } catch (error) {
    controller.abort();
    throw error;
  }
}

async function partDestination(
  transfer: UploadTransfer,
  part: number,
  signal: AbortSignal,
): Promise<PartDestination> {
  const settings = transfer.settings;
  switch (settings.transport) {
    case 'cos': {
      const signed = await transfer.transport.part(transfer.session.id, part, signal);
      return { transport: 'cos', url: signed.url, origin: settings.upload_origin };
    }
    case 'direct':
      return { transport: 'direct', id: transfer.session.id, number: part };
    default:
      return assertNever(settings);
  }
}

function assertNever(_value: never): never {
  throw new BackupTransportError('invalid_response');
}

import {
  backupFileLimit,
  type BackupInspection,
  isRecord,
  parseBackupInspection,
} from './database-backup.types.ts';

export const backupPartSize = 4 * 1024 * 1024;
const uploadStates = [
  'uploading',
  'preparing',
  'preparation_failed',
  'ready',
  'inspecting',
  'inspected',
  'restoring',
  'succeeded',
  'failed',
  'cancelled',
  'expired',
  'unknown',
] as const;
export type UploadState = (typeof uploadStates)[number];
export interface BackupUpload {
  readonly id: string;
  readonly size: number;
  readonly part_size: number;
  readonly part_sha256: readonly string[];
  readonly uploaded_parts: readonly number[];
  readonly uploaded_bytes: number;
  readonly prepared_bytes: number;
  readonly state: UploadState;
  readonly expires_at: string;
  readonly sha256?: string;
  readonly inspection?: BackupInspection;
  readonly job_id?: string;
  readonly error_code?: string;
  readonly cleanup_status: 'not_required' | 'pending' | 'completed';
}
interface UploadLimits {
  readonly part_size: number;
  readonly max_file_size: number;
}
export type BackupUploadSettings = UploadLimits &
  (
    | { readonly transport: 'cos'; readonly enabled: true; readonly upload_origin: string }
    | { readonly transport: 'direct'; readonly enabled: true; readonly upload_origin: '' }
    | { readonly transport: 'disabled'; readonly enabled: false; readonly upload_origin: '' }
  );
export interface CurrentBackupUpload {
  readonly upload: BackupUpload | null;
  readonly settings: BackupUploadSettings;
}
export function parseUploadOrigin(value: unknown): string | null {
  if (typeof value !== 'string') return null;
  const url = URL.parse(value);
  return url &&
    url.protocol === 'https:' &&
    !url.username &&
    !url.password &&
    !url.port &&
    url.origin === value &&
    /^[-a-z0-9]+\.cos\.[-a-z0-9]+\.myqcloud\.com$/u.test(url.hostname)
    ? url.origin
    : null;
}
function isCount(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
}
function isUUID(value: unknown): value is string {
  return typeof value === 'string' && /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/iu.test(value);
}
function isHash(value: unknown): value is string {
  return typeof value === 'string' && /^[0-9a-f]{64}$/u.test(value);
}
function isState(value: unknown): value is UploadState {
  return uploadStates.some((state) => state === value);
}
export function parseBackupUpload(value: unknown): BackupUpload | null {
  if (!isRecord(value)) return null;
  const hashes = value.part_sha256;
  if (
    !isUUID(value.id) ||
    !isCount(value.size) ||
    value.size === 0 ||
    value.size > backupFileLimit ||
    value.part_size !== backupPartSize ||
    !Array.isArray(hashes) ||
    hashes.length !== Math.ceil(value.size / backupPartSize) ||
    !hashes.every(isHash) ||
    !Array.isArray(value.uploaded_parts) ||
    !value.uploaded_parts.every(
      (part: unknown) => isCount(part) && part >= 1 && part <= hashes.length,
    ) ||
    new Set(value.uploaded_parts).size !== value.uploaded_parts.length ||
    !isCount(value.uploaded_bytes) ||
    value.uploaded_bytes > value.size ||
    !isCount(value.prepared_bytes) ||
    value.prepared_bytes > value.size ||
    !isState(value.state) ||
    typeof value.expires_at !== 'string' ||
    !Number.isFinite(Date.parse(value.expires_at)) ||
    (value.sha256 !== undefined && !isHash(value.sha256)) ||
    (value.job_id !== undefined && !isUUID(value.job_id)) ||
    (value.error_code !== undefined &&
      (typeof value.error_code !== 'string' || !/^[a-z_]+$/u.test(value.error_code))) ||
    !['not_required', 'pending', 'completed'].includes(String(value.cleanup_status))
  )
    return null;
  const inspection =
    value.inspection === undefined ? undefined : parseBackupInspection(value.inspection);
  if (inspection === null || (inspection && value.sha256 !== inspection.sha256)) return null;
  const cleanup_status = value.cleanup_status;
  if (
    cleanup_status !== 'not_required' &&
    cleanup_status !== 'pending' &&
    cleanup_status !== 'completed'
  )
    return null;
  return {
    id: value.id,
    size: value.size,
    part_size: value.part_size,
    part_sha256: hashes,
    uploaded_parts: value.uploaded_parts,
    uploaded_bytes: value.uploaded_bytes,
    prepared_bytes: value.prepared_bytes,
    state: value.state,
    expires_at: value.expires_at,
    cleanup_status,
    ...(value.sha256 === undefined ? {} : { sha256: value.sha256 }),
    ...(inspection === undefined ? {} : { inspection }),
    ...(value.job_id === undefined ? {} : { job_id: value.job_id }),
    ...(value.error_code === undefined ? {} : { error_code: value.error_code }),
  };
}
export function parseCurrentBackupUpload(value: unknown): CurrentBackupUpload | null {
  if (!isRecord(value) || !isRecord(value.settings)) return null;
  const settings = parseUploadSettings(value.settings);
  if (!settings) return null;
  const upload = value.upload === null ? null : parseBackupUpload(value.upload);
  if (value.upload !== null && upload === null) return null;
  return {
    upload,
    settings,
  };
}
function parseUploadSettings(value: Record<string, unknown>): BackupUploadSettings | null {
  if (value.part_size !== backupPartSize || value.max_file_size !== backupFileLimit) return null;
  const limits = { part_size: value.part_size, max_file_size: value.max_file_size };
  switch (value.transport) {
    case 'cos': {
      const origin = parseUploadOrigin(value.upload_origin);
      return value.enabled === true && origin
        ? { ...limits, transport: 'cos', enabled: true, upload_origin: origin }
        : null;
    }
    case 'direct':
      return value.enabled === true && value.upload_origin === ''
        ? { ...limits, transport: 'direct', enabled: true, upload_origin: '' }
        : null;
    case 'disabled':
      return value.enabled === false && value.upload_origin === ''
        ? { ...limits, transport: 'disabled', enabled: false, upload_origin: '' }
        : null;
    default:
      return null;
  }
}
export function parsePartURL(
  value: unknown,
): { readonly url: string; readonly expires_at: string } | null {
  if (
    !isRecord(value) ||
    typeof value.url !== 'string' ||
    typeof value.expires_at !== 'string' ||
    !Number.isFinite(Date.parse(value.expires_at))
  )
    return null;
  const url = URL.parse(value.url);
  return url && !url.username && !url.password && parseUploadOrigin(url.origin)
    ? { url: value.url, expires_at: value.expires_at }
    : null;
}

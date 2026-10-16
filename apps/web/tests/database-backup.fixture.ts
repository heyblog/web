import {
  backupDatasetNames,
  type BackupInspection,
} from '../src/api/database-backup/database-backup.types.ts';

export const sourceAdmin = '019f033c-2111-7000-9000-000000000001';
export const targetAdmin = '019f033c-2111-7000-9000-000000000002';
export const inspectionFixture = {
  sha256: 'a'.repeat(64),
  version: 1,
  schema_version: 3,
  generated_at: '2026-10-07T01:00:00Z',
  excluded_system_admin_id: sourceAdmin,
  retained_system_admin_id: targetAdmin,
  datasets: backupDatasetNames.map((name) => ({ name, count: name === 'directory.sites' ? 2 : 0 })),
  graph: { vertices: 3, edges: 2 },
  target_ready: true,
  issues: [],
  can_restore: true,
} satisfies BackupInspection;
export const restorationFixture = {
  status: 'restored',
  sha256: inspectionFixture.sha256,
  datasets: inspectionFixture.datasets,
  graph: inspectionFixture.graph,
  admin_mapping: { source_id: sourceAdmin, target_id: targetAdmin },
};

export const jobFixture = {
  upload_id: '019f033c-2111-7000-9000-000000000004',
  id: '019f033c-2111-7000-9000-000000000003',
  status: 'running',
  progress: { stage: 'validating', dataset: '', completed: 0, total: 0 },
} as const;

export const uploadFixture = {
  id: '019f033c-2111-7000-9000-000000000004',
  size: 2,
  part_size: 4 * 1024 * 1024,
  part_sha256: ['0'.repeat(64)],
  uploaded_parts: [1],
  uploaded_bytes: 2,
  prepared_bytes: 2,
  state: 'inspected',
  expires_at: '2027-01-01T00:00:00Z',
  sha256: inspectionFixture.sha256,
  inspection: inspectionFixture,
  cleanup_status: 'not_required',
} as const;
export const uploadsFixture = {
  upload: uploadFixture,
  settings: {
    transport: 'cos',
    enabled: true,
    upload_origin: 'https://backup-123.cos.ap-singapore.myqcloud.com',
    part_size: 4 * 1024 * 1024,
    max_file_size: 512 * 1024 * 1024,
  },
} as const;

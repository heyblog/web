import { backupDatasetNames } from '../src/api/database-backup/database-backup.types.ts';

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
};
export const restorationFixture = {
  status: 'restored',
  sha256: inspectionFixture.sha256,
  datasets: inspectionFixture.datasets,
  graph: inspectionFixture.graph,
  admin_mapping: { source_id: sourceAdmin, target_id: targetAdmin },
};

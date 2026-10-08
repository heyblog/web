export const backupFileLimit = 512 * 1024 * 1024;
export const backupBodyLimit = backupFileLimit + 1024 * 1024;
export const backupTimeoutMs = 30 * 60 * 1000;

export const backupDatasetNames = [
  'identity.users',
  'identity.oauth_identities',
  'identity.email_verification_codes',
  'identity.password_reset_tokens',
  'identity.user_management_permissions',
  'identity.api_clients',
  'identity.api_client_scopes',
  'identity.api_keys',
  'directory.site_claims',
  'directory.site_ownerships',
  'directory.site_ownership_events',
  'directory.owner_friend_link_requests',
  'directory.sites',
  'directory.site_feeds',
  'directory.site_resources',
  'directory.site_icons',
  'directory.site_tags',
  'directory.software_components',
  'directory.software_component_dependencies',
  'directory.site_software_components',
  'directory.site_sources',
  'directory.site_origins',
  'directory.site_audits',
  'directory.tag_cascades',
  'directory.tag_slug_aliases',
  'directory.tag_identity_aliases',
  'directory.tag_assignment_archive',
  'directory.tag_labels',
  'directory.tags',
  'directory.slug_generation_cache',
  'directory.slug_generation_jobs',
  'directory.site_metrics',
  'directory.site_metric_events',
  'content.announcements',
  'content.announcement_revisions',
  'content.articles',
  'content.article_tags',
  'content.system_ai_settings',
] as const;
export type BackupDatasetName = (typeof backupDatasetNames)[number];
export interface BackupDataset {
  readonly name: BackupDatasetName;
  readonly count: number;
}
export interface BackupGraphCounts {
  readonly vertices: number;
  readonly edges: number;
}
export interface BackupIssue {
  readonly code: string;
  readonly dataset?: string;
}
export interface BackupInspection {
  readonly sha256: string;
  readonly version: 1;
  readonly schema_version: 2;
  readonly generated_at: string;
  readonly excluded_system_admin_id: string;
  readonly retained_system_admin_id: string;
  readonly datasets: readonly BackupDataset[];
  readonly graph: BackupGraphCounts;
  readonly target_ready: boolean;
  readonly issues: readonly BackupIssue[];
  readonly can_restore: boolean;
}
export interface BackupRestoration {
  readonly status: 'restored';
  readonly sha256: string;
  readonly datasets: readonly BackupDataset[];
  readonly graph: BackupGraphCounts;
  readonly admin_mapping: { readonly source_id: string; readonly target_id: string };
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
function isUUID(value: unknown): value is string {
  return typeof value === 'string' && /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/iu.test(value);
}
function isCount(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
}
function isDatasetName(value: unknown): value is BackupDatasetName {
  return backupDatasetNames.some((name) => name === value);
}
function parseDatasets(value: unknown): readonly BackupDataset[] | null {
  if (!Array.isArray(value) || value.length !== backupDatasetNames.length) return null;
  const datasets: BackupDataset[] = [];
  const names = new Set<string>();
  for (const item of value) {
    if (
      !isRecord(item) ||
      !isDatasetName(item.name) ||
      !isCount(item.count) ||
      names.has(item.name)
    )
      return null;
    names.add(item.name);
    datasets.push({ name: item.name, count: item.count });
  }
  return datasets;
}
function parseGraph(value: unknown): BackupGraphCounts | null {
  if (!isRecord(value) || !isCount(value.vertices) || !isCount(value.edges)) return null;
  return { vertices: value.vertices, edges: value.edges };
}
function parseIssues(value: unknown): readonly BackupIssue[] | null {
  if (!Array.isArray(value)) return null;
  const issues: BackupIssue[] = [];
  for (const item of value) {
    if (
      !isRecord(item) ||
      typeof item.code !== 'string' ||
      !/^[a-z_]+$/u.test(item.code) ||
      (item.dataset !== undefined && typeof item.dataset !== 'string')
    )
      return null;
    issues.push({
      code: item.code,
      ...(item.dataset === undefined ? {} : { dataset: item.dataset }),
    });
  }
  return issues;
}
function isHash(value: unknown): value is string {
  return typeof value === 'string' && /^[0-9a-f]{64}$/u.test(value);
}
export function parseBackupInspection(value: unknown): BackupInspection | null {
  if (
    !isRecord(value) ||
    !isHash(value.sha256) ||
    value.version !== 1 ||
    value.schema_version !== 2 ||
    typeof value.generated_at !== 'string' ||
    !Number.isFinite(Date.parse(value.generated_at)) ||
    !isUUID(value.excluded_system_admin_id) ||
    !isUUID(value.retained_system_admin_id) ||
    typeof value.target_ready !== 'boolean' ||
    typeof value.can_restore !== 'boolean'
  )
    return null;
  const datasets = parseDatasets(value.datasets);
  const graph = parseGraph(value.graph);
  const issues = parseIssues(value.issues);
  if (
    !datasets ||
    !graph ||
    !issues ||
    (value.can_restore && (!value.target_ready || issues.length > 0))
  )
    return null;
  return {
    sha256: value.sha256,
    version: 1,
    schema_version: 2,
    generated_at: value.generated_at,
    excluded_system_admin_id: value.excluded_system_admin_id,
    retained_system_admin_id: value.retained_system_admin_id,
    datasets,
    graph,
    issues,
    target_ready: value.target_ready,
    can_restore: value.can_restore,
  };
}
export function parseBackupRestoration(value: unknown): BackupRestoration | null {
  if (
    !isRecord(value) ||
    value.status !== 'restored' ||
    !isHash(value.sha256) ||
    !isRecord(value.admin_mapping) ||
    !isUUID(value.admin_mapping.source_id) ||
    !isUUID(value.admin_mapping.target_id)
  )
    return null;
  const datasets = parseDatasets(value.datasets);
  const graph = parseGraph(value.graph);
  if (!datasets || !graph) return null;
  return {
    status: 'restored',
    sha256: value.sha256,
    datasets,
    graph,
    admin_mapping: {
      source_id: value.admin_mapping.source_id,
      target_id: value.admin_mapping.target_id,
    },
  };
}

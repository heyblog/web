export const auditActions = ['CREATE', 'UPDATE', 'DELETE', 'RESTORE'] as const;
export type AuditAction = (typeof auditActions)[number];
export const auditStatuses = ['PENDING', 'APPROVED', 'REJECTED'] as const;
export type AuditStatus = (typeof auditStatuses)[number];
export const dependencyRoles = ['FRAMEWORK', 'LANGUAGE'] as const;
export type DependencyRole = (typeof dependencyRoles)[number];
export const feedFormats = ['UNKNOWN', 'RSS', 'ATOM', 'JSON'] as const;
export type FeedFormat = (typeof feedFormats)[number];

export interface Option {
  readonly id: string;
  readonly name: string;
}
export interface ComponentOption extends Option {
  readonly homepage_url: string;
  readonly repository_url: string;
  readonly is_open_source: boolean;
}
export interface ProgramDependencyOption {
  readonly program_id: string;
  readonly component_id: string;
  readonly role: DependencyRole | 'RUNTIME' | 'OTHER';
}
export interface SubmissionOptions {
  readonly tags: readonly Option[];
  readonly components: readonly ComponentOption[];
  readonly program_dependencies: readonly ProgramDependencyOption[];
  readonly private_program_id: string;
}
export interface FeedInput {
  readonly name: string;
  readonly url: string;
  readonly format: FeedFormat;
  readonly is_default: boolean;
}
export interface ResourceInput {
  readonly kind: 'SITEMAP' | 'LINK_PAGE';
  readonly url: string;
}
export interface TagInput {
  readonly id: string;
  readonly suggested_name: string;
  readonly slug: string;
  readonly description: string;
  readonly role: 'PRIMARY' | 'SECONDARY';
}
export interface ComponentInput {
  readonly id: string;
  readonly suggested_name: string;
  readonly role: 'SITE_PROGRAM' | DependencyRole | 'RUNTIME' | 'OTHER';
  readonly homepage_url: string;
  readonly repository_url: string;
  readonly is_open_source: boolean | null;
}
export interface SiteInput {
  readonly name: string;
  readonly url: string;
  readonly summary: string;
  readonly feeds: readonly FeedInput[];
  readonly resources: readonly ResourceInput[];
  readonly tags: readonly TagInput[];
  readonly components: readonly ComponentInput[];
  readonly program_dependencies: readonly ComponentInput[];
}
export interface SubmissionPayload {
  readonly site: SiteInput;
  readonly reason?: string;
  readonly contact: {
    readonly name: string;
    readonly email: string;
    readonly notify_by_email: boolean;
  };
}
export interface TagSnapshot extends TagInput {
  readonly name: string;
}
export interface ComponentSnapshot extends ComponentInput {
  readonly name: string;
}
export type Snapshot = Omit<SiteInput, 'url' | 'tags' | 'components' | 'program_dependencies'> & {
  readonly site_id?: string;
  readonly revision?: number;
  readonly short_id?: string;
  readonly custom_id?: string;
  readonly scheme: string;
  readonly normalized_host: string;
  readonly base_path: string;
  readonly access_scope: string;
  readonly visibility: string;
  readonly visibility_reason?: string;
  readonly tags: readonly TagSnapshot[];
  readonly components: readonly ComponentSnapshot[];
  readonly program_dependencies: readonly ComponentSnapshot[];
};
export type PublicSnapshot = Omit<Snapshot, 'site_id'>;
export interface SubmissionResult {
  readonly audit_id: string;
  readonly lookup_token: string;
  readonly action: AuditAction;
  readonly status: AuditStatus;
  readonly short_id?: string;
}
export interface PublicAuditResult {
  readonly action: AuditAction;
  readonly status: AuditStatus;
  readonly short_id?: string;
  readonly reviewer_comment?: string;
  readonly reviewed_at?: string;
  readonly created_at: string;
}
export interface DiffChange {
  readonly key: string;
  readonly before: string;
  readonly after: string;
}
export interface DiffItem {
  readonly field: string;
  readonly before?: string;
  readonly after?: string;
  readonly added?: readonly string[];
  readonly removed?: readonly string[];
  readonly changed?: readonly DiffChange[];
}
export interface DiffViews {
  readonly requested: readonly DiffItem[];
  readonly drift: readonly DiffItem[];
  readonly reviewer_correction: readonly DiffItem[];
  readonly conflicts: readonly DiffItem[];
}
export interface AuditDetail {
  readonly id: string;
  readonly action: AuditAction;
  readonly status: AuditStatus;
  readonly site_id?: string;
  readonly base_revision?: number;
  readonly base_snapshot: Snapshot;
  readonly proposed_snapshot: Snapshot;
  readonly review_draft_snapshot?: Snapshot;
  readonly review_draft_revision: number;
  readonly review_draft_updated_by?: string;
  readonly review_draft_updated_at?: string;
  readonly final_snapshot: Snapshot;
  readonly current_snapshot: Snapshot;
  readonly effective_snapshot: Snapshot;
  readonly request_reason: string;
  readonly submitter_name?: string;
  readonly submitter_email?: string;
  readonly notify_by_email: boolean;
  readonly reviewer_comment?: string;
  readonly reviewed_at?: string;
  readonly created_at: string;
  readonly diff: DiffViews;
}
export interface AuditListItem {
  readonly id: string;
  readonly action: AuditAction;
  readonly status: AuditStatus;
  readonly site_id?: string;
  readonly site_name: string;
  readonly site_address: string;
  readonly submitter_name?: string;
  readonly submitter_email?: string;
  readonly reviewed_at?: string;
  readonly created_at: string;
}
export interface AuditPage {
  readonly items: readonly AuditListItem[];
  readonly page: number;
  readonly page_size: number;
  readonly total_items: number;
  readonly total_pages: number;
}
export interface SiteSearchResult {
  readonly short_id: string;
  readonly name: string;
  readonly url: string;
  readonly visibility: string;
}

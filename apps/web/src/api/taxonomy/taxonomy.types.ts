export interface ManagedTag {
  readonly id: string;
  readonly name: string;
  readonly slug: string;
  readonly description: string;
  readonly roles: readonly string[];
  readonly is_enabled: boolean;
  readonly site_count: number;
  readonly article_count: number;
}

export interface ManagedCascade {
  readonly id: string;
  readonly scope: 'SITE' | 'ARTICLE';
  readonly level1_tag_id: string;
  readonly level2_tag_id: string;
  readonly taxonomy_key: string;
  readonly is_enabled: boolean;
  readonly merged_into_id: string;
}

export interface TaxonomyData {
  readonly tags: readonly ManagedTag[];
  readonly cascades: readonly ManagedCascade[];
  readonly revision: string;
}

export type ChangeKind = 'merge' | 'path_update' | 'path_merge';

export interface TaxonomyChange {
  readonly kind: ChangeKind;
  readonly source_id: string;
  readonly target_id?: string;
  readonly primary_id?: string;
  readonly secondary_id?: string;
  readonly is_enabled?: boolean;
  readonly expected_revision: string;
}

export interface ChangePreview {
  readonly revision: string;
  readonly fingerprint: string;
  readonly site_count: number;
  readonly article_count: number;
  readonly removed_duplicates: number;
  readonly blockers: readonly string[];
  readonly paths: readonly {
    readonly cascade_id: string;
    readonly scope: string;
    readonly label: string;
  }[];
}

export interface SlugInput {
  readonly name: string;
  readonly description: string;
  readonly parent_name: string;
  readonly tag_id: string;
}

export interface GeneratedSlug {
  readonly slug: string;
  readonly source: 'local' | 'ai' | 'cache';
  readonly model_id: string;
}

export function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function isUUID(value: unknown): value is string {
  return (
    typeof value === 'string' &&
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value)
  );
}

export function isRevision(value: unknown): value is string {
  return (
    typeof value === 'string' && /^\d{1,19}$/.test(value) && BigInt(value) <= 9223372036854775807n
  );
}

export function isTaxonomyRevision(value: unknown): value is string {
  return typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
}

function optionalID(value: unknown): value is string {
  return value === '' || isUUID(value);
}

function isCount(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
}

function isTag(value: unknown): value is ManagedTag {
  return (
    isRecord(value) &&
    isUUID(value.id) &&
    typeof value.name === 'string' &&
    typeof value.slug === 'string' &&
    typeof value.description === 'string' &&
    Array.isArray(value.roles) &&
    value.roles.every(
      (role: unknown) =>
        typeof role === 'string' && ['PRIMARY', 'SECONDARY', 'TERTIARY', 'WARNING'].includes(role),
    ) &&
    typeof value.is_enabled === 'boolean' &&
    isCount(value.site_count) &&
    isCount(value.article_count)
  );
}

function isCascade(value: unknown): value is ManagedCascade {
  return (
    isRecord(value) &&
    isUUID(value.id) &&
    (value.scope === 'SITE' || value.scope === 'ARTICLE') &&
    isUUID(value.level1_tag_id) &&
    isUUID(value.level2_tag_id) &&
    typeof value.taxonomy_key === 'string' &&
    typeof value.is_enabled === 'boolean' &&
    optionalID(value.merged_into_id)
  );
}

export function parseTaxonomy(value: unknown): TaxonomyData | null {
  return isRecord(value) &&
    Array.isArray(value.tags) &&
    value.tags.every(isTag) &&
    Array.isArray(value.cascades) &&
    value.cascades.every(isCascade) &&
    isTaxonomyRevision(value.revision)
    ? { tags: value.tags, cascades: value.cascades, revision: value.revision }
    : null;
}

export function parsePreview(value: unknown): ChangePreview | null {
  const isPath = (path: unknown): path is ChangePreview['paths'][number] =>
    isRecord(path) &&
    isUUID(path.cascade_id) &&
    (path.scope === 'SITE' || path.scope === 'ARTICLE') &&
    typeof path.label === 'string';
  return isRecord(value) &&
    isTaxonomyRevision(value.revision) &&
    typeof value.fingerprint === 'string' &&
    isCount(value.site_count) &&
    isCount(value.article_count) &&
    isCount(value.removed_duplicates) &&
    Array.isArray(value.blockers) &&
    value.blockers.every((item) => typeof item === 'string') &&
    Array.isArray(value.paths) &&
    value.paths.every(isPath)
    ? {
        revision: value.revision,
        fingerprint: value.fingerprint,
        site_count: value.site_count,
        article_count: value.article_count,
        removed_duplicates: value.removed_duplicates,
        blockers: value.blockers,
        paths: value.paths,
      }
    : null;
}

export function parseSlug(value: unknown): GeneratedSlug | null {
  return isRecord(value) &&
    typeof value.slug === 'string' &&
    /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(value.slug) &&
    (value.source === 'local' || value.source === 'ai' || value.source === 'cache') &&
    typeof value.model_id === 'string'
    ? { slug: value.slug, source: value.source, model_id: value.model_id }
    : null;
}

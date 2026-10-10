import { isRecord, isTaxonomyRevision, isUUID } from './taxonomy.types.ts';

const stringValue = (value: unknown, max: number): value is string =>
  typeof value === 'string' && Array.from(value).length <= max;

function fields(value: Readonly<Record<string, unknown>>, allowed: readonly string[]): boolean {
  return Object.keys(value).every((key) => allowed.includes(key));
}

export function validManagementPayload(
  value: unknown,
  path: string,
  method: string,
): value is Readonly<Record<string, unknown>> {
  if (!isRecord(value)) return false;
  if (path === 'cascades')
    return (
      fields(value, ['scope', 'primary_id', 'secondary_id', 'taxonomy_key', 'expected_revision']) &&
      (value.scope === 'SITE' || value.scope === 'ARTICLE') &&
      isUUID(value.primary_id) &&
      isUUID(value.secondary_id) &&
      stringValue(value.taxonomy_key, 160) &&
      value.taxonomy_key.trim() !== '' &&
      isTaxonomyRevision(value.expected_revision)
    );
  if (path === 'changes/preview' || path === 'changes/apply') {
    return (
      fields(value, [
        'kind',
        'source_id',
        'target_id',
        'primary_id',
        'secondary_id',
        'is_enabled',
        'expected_revision',
        ...(path.endsWith('apply') ? ['fingerprint'] : []),
      ]) &&
      isUUID(value.source_id) &&
      isTaxonomyRevision(value.expected_revision) &&
      (value.kind === 'path_update'
        ? isUUID(value.primary_id) &&
          isUUID(value.secondary_id) &&
          typeof value.is_enabled === 'boolean'
        : (value.kind === 'merge' || value.kind === 'path_merge') && isUUID(value.target_id)) &&
      (path.endsWith('preview') ||
        (stringValue(value.fingerprint, 128) && value.fingerprint !== ''))
    );
  }
  if (method === 'DELETE')
    return fields(value, ['expected_revision']) && isTaxonomyRevision(value.expected_revision);
  const creating = path === '';
  return (
    fields(
      value,
      creating
        ? ['name', 'description', 'expected_revision']
        : ['name', 'description', 'is_enabled', 'expected_revision'],
    ) &&
    stringValue(value.name, 120) &&
    value.name.trim() !== '' &&
    stringValue(value.description, 2000) &&
    isTaxonomyRevision(value.expected_revision) &&
    (creating || typeof value.is_enabled === 'boolean')
  );
}

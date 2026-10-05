import { isRecord, isRevision, isTaxonomyRevision, isUUID } from './taxonomy.types.ts';
const fields = (value: Readonly<Record<string, unknown>>, allowed: readonly string[]) =>
  Object.keys(value).every((key) => allowed.includes(key));
const ids = (value: unknown): boolean =>
  Array.isArray(value) &&
  value.length > 0 &&
  value.length <= 500 &&
  value.every(isUUID) &&
  new Set(value).size === value.length;
export function slugJobMethods(path: string): readonly string[] {
  if (path === 'slug-jobs') return ['GET', 'POST'];
  const [root, id, action, extra] = path.split('/');
  if (root !== 'slug-jobs' || !isUUID(id) || extra !== undefined) return [];
  if (action === undefined) return ['GET'];
  if (action === 'items') return ['PATCH'];
  if (action === 'control' || action === 'apply') return ['POST'];
  return [];
}
export function validSlugJobPayload(
  value: unknown,
  path: string,
): value is Readonly<Record<string, unknown>> {
  if (!isRecord(value)) return false;
  if (path === 'slug-jobs') {
    if (
      !fields(value, ['selection', 'expected_revision']) ||
      (value.expected_revision !== undefined && !isTaxonomyRevision(value.expected_revision)) ||
      !isRecord(value.selection)
    )
      return false;
    const selection = value.selection;
    if (selection.kind === 'all' || selection.kind === 'invalid')
      return fields(selection, ['kind']);
    if (selection.kind === 'ids') return fields(selection, ['kind', 'ids']) && ids(selection.ids);
    if (selection.kind === 'filter')
      return (
        fields(selection, ['kind', 'filter']) &&
        isRecord(selection.filter) &&
        fields(selection.filter, ['query', 'is_enabled']) &&
        (selection.filter.query === undefined ||
          (typeof selection.filter.query === 'string' && selection.filter.query.length <= 128)) &&
        (selection.filter.is_enabled === undefined ||
          typeof selection.filter.is_enabled === 'boolean')
      );
    return false;
  }
  if (path.endsWith('/control'))
    return (
      fields(value, ['action', 'expected_revision']) &&
      (value.expected_revision === undefined || isRevision(value.expected_revision)) &&
      ['pause', 'resume', 'cancel', 'retry'].includes(String(value.action))
    );
  if (!isRevision(value.expected_revision)) return false;
  if (path.endsWith('/apply'))
    return fields(value, ['tag_ids', 'expected_revision']) && ids(value.tag_ids);
  if (path.endsWith('/items'))
    return (
      fields(value, ['items', 'expected_revision']) &&
      Array.isArray(value.items) &&
      value.items.length > 0 &&
      value.items.length <= 500 &&
      value.items.every(
        (item) =>
          isRecord(item) &&
          fields(item, ['tag_id', 'slug']) &&
          isUUID(item.tag_id) &&
          typeof item.slug === 'string' &&
          item.slug.length <= 128 &&
          /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(item.slug),
      ) &&
      new Set(value.items.map((item: unknown) => (isRecord(item) ? item.tag_id : undefined)))
        .size === value.items.length
    );
  return false;
}

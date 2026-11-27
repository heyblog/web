import type {
  AnnotationDetail,
  AnnotationFields,
  AnnotationPatch,
} from '../../api/site-annotation/annotation.types.ts';

type Mutable<T> = { -readonly [K in keyof T]: T[K] };
type DraftRow<T> = Mutable<T> & { readonly key: number };

function rows<T>(items: readonly T[]): DraftRow<T>[] {
  return items.map((item, key) => ({ ...item, key }));
}

export function annotationDraft(detail: AnnotationDetail) {
  return {
    name: detail.name,
    url: detail.url,
    summary: detail.summary,
    blog_status: detail.blog_status,
    tag_cascade_id: detail.tag_cascade_id,
    tags: rows(
      detail.tags.flatMap((tag) =>
        tag.role === 'TERTIARY' || tag.role === 'WARNING' ? [{ ...tag, role: tag.role }] : [],
      ),
    ),
    components: rows(detail.components),
    feeds: rows<AnnotationFields['feeds'][number]>(detail.feeds),
    resources: rows(detail.resources),
    outgoing: rows<AnnotationFields['outgoing'][number]>(detail.outgoing),
  };
}

export type AnnotationDraft = ReturnType<typeof annotationDraft>;

function fields(draft: AnnotationDraft): AnnotationFields {
  return {
    name: draft.name,
    url: draft.url,
    summary: draft.summary,
    blog_status: draft.blog_status,
    tag_cascade_id: draft.tag_cascade_id,
    tags: draft.tags.map(({ id, role }) => ({ id, role })),
    components: draft.components.map(({ id, role }) => ({ id, role })),
    feeds: draft.feeds.map(({ id, name, url, format, is_default, is_enabled }) => ({
      ...(id ? { id } : {}),
      name,
      url,
      format,
      is_default,
      is_enabled,
    })),
    resources: draft.resources.map(({ kind, url }) => ({ kind, url })),
    outgoing: draft.outgoing.map(({ target_url, link_status }) => ({ target_url, link_status })),
  };
}

export function annotationPatch(
  draft: AnnotationDraft,
  detail: AnnotationDetail,
  reason: string,
): AnnotationPatch | null {
  const current = fields(draft);
  const saved = fields(annotationDraft(detail));
  const changes: Mutable<Partial<AnnotationFields>> = {};
  function compare<K extends keyof AnnotationFields>(key: K) {
    if (JSON.stringify(current[key]) !== JSON.stringify(saved[key])) changes[key] = current[key];
  }
  (Object.keys(current) as (keyof AnnotationFields)[]).forEach(compare);
  if (Object.keys(changes).length === 0) return null;
  return {
    ...changes,
    expected_revision: detail.revision,
    friend_version: detail.friend_version,
    ...(reason.trim() ? { reason: reason.trim() } : {}),
  };
}

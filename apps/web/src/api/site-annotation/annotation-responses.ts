import { feedFormats, type Option } from '../site-submission/site-submission.types.ts';

import {
  accessScopes,
  type AnnotationDetail,
  type AnnotationHistoryItem,
  type AnnotationItem,
  type AnnotationOptions,
  type AnnotationPage,
  blogStatuses,
  componentRoles,
  historySources,
  linkStatuses,
  resourceKinds,
  tagRoles,
} from './annotation.types.ts';
import { isAnnotationID } from './annotation-contract.ts';
import { bool, int, list, oneOf, record, text, timestamp } from './annotation-parsing.ts';

function id(value: unknown): string {
  const parsed = text(value);
  if (!isAnnotationID(parsed)) throw new TypeError('Invalid annotation identifier');
  return parsed;
}

function option(value: unknown): Pick<Option, 'id' | 'name' | 'level'> {
  const item = record(value);
  const level = item.level;
  if (level != null && level !== 1 && level !== 2 && level !== 3)
    throw new TypeError('Invalid tag level');
  return { id: id(item.id), name: text(item.name), ...(level == null ? {} : { level }) };
}

function annotationItem(value: unknown): AnnotationItem {
  const item = record(value);
  return {
    short_id: id(item.short_id),
    name: text(item.name),
    url: text(item.url),
    blog_status: oneOf(item.blog_status, blogStatuses),
    revision: int(item.revision),
    updated_at: timestamp(item.updated_at),
    ...(item.pending_audit_id ? { pending_audit_id: id(item.pending_audit_id) } : {}),
  };
}

export function parseAnnotationDetail(value: unknown): AnnotationDetail {
  const item = record(value);
  return {
    ...annotationItem(value),
    summary: text(item.summary),
    access_scope: oneOf(item.access_scope, accessScopes),
    tag_cascade_id:
      item.tag_cascade_id == null || item.tag_cascade_id === '' ? '' : id(item.tag_cascade_id),
    tags: list(item.tags ?? [], (value) => ({
      ...option(value),
      role: oneOf(record(value).role, [...tagRoles, 'PRIMARY', 'SECONDARY']),
    })),
    components: list(item.components ?? [], (value) => {
      const entry = record(value);
      return { id: id(entry.id), name: text(entry.name), role: oneOf(entry.role, componentRoles) };
    }),
    feeds: list(item.feeds ?? [], (value) => {
      const entry = record(value);
      return {
        id: id(entry.id),
        name: text(entry.name),
        url: text(entry.url),
        format: oneOf(entry.format, feedFormats),
        is_default: bool(entry.is_default),
        is_enabled: bool(entry.is_enabled),
      };
    }),
    resources: list(item.resources ?? [], (value) => {
      const entry = record(value);
      return { kind: oneOf(entry.kind, resourceKinds), url: text(entry.url) };
    }),
    outgoing: list(item.outgoing ?? [], (value) => {
      const entry = record(value);
      return {
        target_host: text(entry.target_host),
        target_url: text(entry.target_url),
        link_status: oneOf(entry.link_status, linkStatuses),
      };
    }),
    incoming: list(item.incoming ?? [], (value) => {
      const entry = record(value);
      return {
        source_short_id: entry.source_short_id ? id(entry.source_short_id) : '',
        source_name: text(entry.source_name),
        source_url: text(entry.source_url),
        link_status: oneOf(entry.link_status, linkStatuses),
      };
    }),
    friend_version: text(item.friend_version),
  };
}

function parsePage<T>(value: unknown, parseItem: (item: unknown) => T): AnnotationPage<T> {
  const page = record(value);
  return {
    items: list(page.items ?? [], parseItem),
    page: int(page.page, 1),
    page_size: int(page.page_size, 1),
    total: int(page.total),
  };
}

export function parseAnnotationPage(value: unknown): AnnotationPage<AnnotationItem> {
  return parsePage(value, annotationItem);
}

export function parseAnnotationHistory(value: unknown): AnnotationPage<AnnotationHistoryItem> {
  return parsePage(value, (value) => {
    const item = record(value);
    return {
      id: id(item.id),
      actor_name: text(item.actor_name),
      created_at: timestamp(item.created_at),
      source: oneOf(item.source, historySources),
      ...(item.reason == null ? {} : { reason: text(item.reason) }),
      before: record(item.before ?? {}),
      after: record(item.after ?? {}),
    };
  });
}

export function parseAnnotationOptions(value: unknown): AnnotationOptions {
  const options = record(value);
  return {
    tags: list(options.tags, option),
    components: list(options.components, option),
    cascades: list(options.cascades ?? [], (value) => {
      const cascade = record(value);
      return { id: id(cascade.id), level1: option(cascade.level1), level2: option(cascade.level2) };
    }),
  };
}

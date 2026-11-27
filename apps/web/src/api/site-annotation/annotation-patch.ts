import { feedFormats } from '../site-submission/site-submission.types.ts';

import {
  accessScopes,
  type AnnotationFields,
  type AnnotationPatch,
  blogStatuses,
  componentRoles,
  linkStatuses,
  resourceKinds,
  tagRoles,
} from './annotation.types.ts';
import { bool, int, list, oneOf, record, text } from './annotation-parsing.ts';

export function parseAnnotationPatch(value: unknown): AnnotationPatch {
  const input = record(value);
  return {
    expected_revision: int(input.expected_revision, 1),
    friend_version: text(input.friend_version),
    ...(input.reason === undefined ? {} : { reason: text(input.reason) }),
    ...parseSections(input),
  };
}

function parseSections(input: Record<string, unknown>): Partial<AnnotationFields> {
  return {
    ...(input.name === undefined ? {} : { name: text(input.name) }),
    ...(input.url === undefined ? {} : { url: text(input.url) }),
    ...(input.summary === undefined ? {} : { summary: text(input.summary) }),
    ...(input.access_scope === undefined
      ? {}
      : { access_scope: oneOf(input.access_scope, accessScopes) }),
    ...(input.blog_status === undefined
      ? {}
      : { blog_status: oneOf(input.blog_status, blogStatuses) }),
    ...(input.tag_cascade_id === undefined ? {} : { tag_cascade_id: text(input.tag_cascade_id) }),
    ...(input.tags === undefined
      ? {}
      : {
          tags: list(input.tags, (value) => {
            const item = record(value);
            return { id: text(item.id), role: oneOf(item.role, tagRoles) };
          }),
        }),
    ...(input.components === undefined
      ? {}
      : {
          components: list(input.components, (value) => {
            const item = record(value);
            return { id: text(item.id), role: oneOf(item.role, componentRoles) };
          }),
        }),
    ...(input.feeds === undefined
      ? {}
      : {
          feeds: list(input.feeds, (value) => {
            const item = record(value);
            return {
              ...(item.id === undefined ? {} : { id: text(item.id) }),
              name: text(item.name),
              url: text(item.url),
              format: oneOf(item.format, feedFormats),
              is_default: bool(item.is_default),
              is_enabled: bool(item.is_enabled),
            };
          }),
        }),
    ...(input.resources === undefined
      ? {}
      : {
          resources: list(input.resources, (value) => {
            const item = record(value);
            return { kind: oneOf(item.kind, resourceKinds), url: text(item.url) };
          }),
        }),
    ...(input.outgoing === undefined
      ? {}
      : {
          outgoing: list(input.outgoing, (value) => {
            const item = record(value);
            return {
              target_url: text(item.target_url),
              link_status: oneOf(item.link_status, linkStatuses),
            };
          }),
        }),
  };
}

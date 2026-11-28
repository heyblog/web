import type {
  ComponentInput,
  FeedInput,
  Option,
  ResourceInput,
} from '../site-submission/site-submission.types.ts';

export const blogStatuses = ['UNREVIEWED', 'BLOG', 'NON_BLOG'] as const;
export const annotationFilters = [...blogStatuses, 'ALL'] as const;
export const accessScopes = ['ALL', 'CN', 'GL'] as const;
export const tagRoles = ['TERTIARY', 'WARNING'] as const;
export const componentRoles = [
  'SITE_PROGRAM',
  'FRAMEWORK',
  'LANGUAGE',
  'RUNTIME',
  'OTHER',
] as const;
export const resourceKinds = ['SITEMAP', 'LINK_PAGE'] as const;
export const linkStatuses = ['ACTIVE', 'INACTIVE'] as const;
export const historySources = ['ANNOTATION', 'AUDIT'] as const;

export interface AnnotationItem {
  readonly short_id: string;
  readonly name: string;
  readonly url: string;
  readonly blog_status: (typeof blogStatuses)[number];
  readonly revision: number;
  readonly updated_at: string;
  readonly pending_audit_id?: string;
}

export type AnnotationTagRole = (typeof tagRoles)[number] | 'PRIMARY' | 'SECONDARY';
export interface AnnotationTag {
  readonly id: string;
  readonly name: string;
  readonly role: AnnotationTagRole;
  readonly level?: Option['level'];
}
export interface AnnotationComponent {
  readonly id: string;
  readonly name: string;
  readonly role: ComponentInput['role'];
}
export interface AnnotationFeed extends FeedInput {
  readonly id: string;
  readonly is_enabled: boolean;
}
export interface OutgoingLink {
  readonly target_host: string;
  readonly target_url: string;
  readonly link_status: (typeof linkStatuses)[number];
}
export interface IncomingLink {
  readonly source_short_id: string;
  readonly source_name: string;
  readonly source_url: string;
  readonly link_status: OutgoingLink['link_status'];
}
export interface AnnotationDetail extends AnnotationItem {
  readonly summary: string;
  readonly access_scope: (typeof accessScopes)[number] | null;
  readonly tag_cascade_id: string;
  readonly tags: readonly AnnotationTag[];
  readonly components: readonly AnnotationComponent[];
  readonly feeds: readonly AnnotationFeed[];
  readonly resources: readonly ResourceInput[];
  readonly outgoing: readonly OutgoingLink[];
  readonly incoming: readonly IncomingLink[];
  readonly friend_version: string;
}

export interface AnnotationPage<T> {
  readonly items: readonly T[];
  readonly page: number;
  readonly page_size: number;
  readonly total: number;
}

export interface AnnotationHistoryItem {
  readonly id: string;
  readonly actor_name: string;
  readonly created_at: string;
  readonly reason?: string;
  readonly source: (typeof historySources)[number];
  readonly before: Readonly<Record<string, unknown>>;
  readonly after: Readonly<Record<string, unknown>>;
}

export interface AnnotationOptions {
  readonly tags: readonly Option[];
  readonly components: readonly Pick<AnnotationComponent, 'id' | 'name'>[];
  readonly cascades: readonly {
    readonly id: string;
    readonly level1: Pick<Option, 'id' | 'name'>;
    readonly level2: Pick<Option, 'id' | 'name'>;
  }[];
}

export type FeedPatch = Omit<AnnotationFeed, 'id'> & { readonly id?: string };
export interface AnnotationFields {
  readonly name: string;
  readonly url: string;
  readonly summary: string;
  readonly blog_status: AnnotationItem['blog_status'];
  readonly tag_cascade_id: string;
  readonly tags: readonly (Pick<AnnotationTag, 'id'> & {
    readonly role: (typeof tagRoles)[number];
  })[];
  readonly components: readonly Pick<AnnotationComponent, 'id' | 'role'>[];
  readonly feeds: readonly FeedPatch[];
  readonly resources: readonly ResourceInput[];
  readonly outgoing: readonly Pick<OutgoingLink, 'target_url' | 'link_status'>[];
}

export interface AnnotationPatch extends Partial<AnnotationFields> {
  readonly expected_revision: number;
  readonly friend_version: AnnotationDetail['friend_version'];
  readonly reason?: string;
}

export interface AnnotationStats {
  readonly total: number;
  readonly annotated: number;
  readonly unreviewed: number;
  readonly blog: number;
  readonly non_blog: number;
  readonly actor_annotated: number;
  readonly actor_today_annotated: number;
}

export interface AnnotationNext {
  readonly short_id: string;
}

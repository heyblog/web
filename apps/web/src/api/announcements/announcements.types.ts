export type AnnouncementKind = 'MAIN' | 'BANNER';
export type AnnouncementStatus = 'DRAFT' | 'PUBLISHED' | 'ARCHIVED';
export type EffectiveStatus = 'DRAFT' | 'SCHEDULED' | 'ACTIVE' | 'EXPIRED' | 'ARCHIVED';
export type ActionType = 'NONE' | 'INTERNAL' | 'EXTERNAL';

export interface AnnouncementAction {
  readonly label: string;
  readonly href: string;
  readonly external: boolean;
}

export interface PublicAnnouncement {
  readonly id: string;
  readonly title: string;
  readonly bodyMarkdown: string | null;
  readonly startsAt: string;
  readonly endsAt: string | null;
  readonly action: AnnouncementAction | null;
}

export interface AnnouncementFields {
  readonly kind: AnnouncementKind;
  readonly title: string;
  readonly bodyMarkdown: string | null;
  readonly priority: number;
  readonly actionType: ActionType;
  readonly actionLabel: string | null;
  readonly actionPath: string | null;
  readonly actionExternalUrl: string | null;
  readonly startsAt: string | null;
  readonly endsAt: string | null;
}

export interface ManagedAnnouncement extends AnnouncementFields {
  readonly id: string;
  readonly status: AnnouncementStatus;
  readonly effectiveStatus: EffectiveStatus;
  readonly rowVersion: string;
  readonly createdAt: string;
  readonly updatedAt: string;
  readonly publishedAt: string | null;
  readonly archivedAt: string | null;
}

export interface AnnouncementRevision extends AnnouncementFields {
  readonly revision: string;
  readonly publishedAt: string;
  readonly changedAt: string;
}

export interface AnnouncementList<T> {
  readonly announcements: readonly T[];
  readonly total: number;
  readonly page: number;
  readonly pageSize: number;
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

export function isVersion(value: unknown): value is string {
  return (
    typeof value === 'string' &&
    /^[1-9]\d{0,18}$/.test(value) &&
    BigInt(value) <= 9223372036854775807n
  );
}

export function safeAnnouncementLink(value: string, external: boolean): boolean {
  if (
    /[\s\\]/.test(value) ||
    Array.from(value).some(
      (character) => character.charCodeAt(0) < 32 || character.charCodeAt(0) === 127,
    )
  )
    return false;
  try {
    const url = new URL(value, 'https://www.heyblog.net');
    if (external)
      return (
        /^https?:\/\//.test(value) &&
        (url.protocol === 'https:' || url.protocol === 'http:') &&
        !url.username &&
        !url.password
      );
    return (
      value.startsWith('/') && !value.startsWith('//') && url.origin === 'https://www.heyblog.net'
    );
  } catch (error) {
    if (error instanceof TypeError) return false;
    throw error;
  }
}

export function isDate(value: unknown): value is string {
  return (
    typeof value === 'string' &&
    /T.*(?:Z|[+-]\d{2}:\d{2})$/.test(value) &&
    Number.isFinite(Date.parse(value))
  );
}

function nullableString(value: unknown): value is string | null {
  return value === null || typeof value === 'string';
}

export function isFields(value: unknown): value is AnnouncementFields {
  if (!isRecord(value)) return false;
  return (
    (value.kind === 'MAIN' || value.kind === 'BANNER') &&
    typeof value.title === 'string' &&
    value.title.trim().length > 0 &&
    Array.from(value.title).length <= 256 &&
    nullableString(value.bodyMarkdown) &&
    Number.isInteger(value.priority) &&
    typeof value.priority === 'number' &&
    value.priority >= -2147483648 &&
    value.priority <= 2147483647 &&
    (value.kind === 'MAIN' || value.priority === 0) &&
    (value.actionType === 'NONE' ||
      value.actionType === 'INTERNAL' ||
      value.actionType === 'EXTERNAL') &&
    nullableString(value.actionLabel) &&
    nullableString(value.actionPath) &&
    nullableString(value.actionExternalUrl) &&
    (value.startsAt === null || isDate(value.startsAt)) &&
    (value.endsAt === null || isDate(value.endsAt)) &&
    (value.endsAt === null ||
      (typeof value.startsAt === 'string' &&
        Date.parse(value.endsAt) > Date.parse(value.startsAt))) &&
    (value.actionType === 'NONE'
      ? value.actionLabel === null && value.actionPath === null && value.actionExternalUrl === null
      : typeof value.actionLabel === 'string' &&
        value.actionLabel.trim().length > 0 &&
        (value.actionType === 'INTERNAL'
          ? typeof value.actionPath === 'string' &&
            safeAnnouncementLink(value.actionPath, false) &&
            value.actionExternalUrl === null
          : typeof value.actionExternalUrl === 'string' &&
            safeAnnouncementLink(value.actionExternalUrl, true) &&
            value.actionPath === null))
  );
}

export function isManagedAnnouncement(value: unknown): value is ManagedAnnouncement {
  return (
    isFields(value) &&
    isRecord(value) &&
    isUUID(value.id) &&
    (value.status === 'DRAFT' || value.status === 'PUBLISHED' || value.status === 'ARCHIVED') &&
    (value.effectiveStatus === 'DRAFT' ||
      value.effectiveStatus === 'SCHEDULED' ||
      value.effectiveStatus === 'ACTIVE' ||
      value.effectiveStatus === 'EXPIRED' ||
      value.effectiveStatus === 'ARCHIVED') &&
    isVersion(value.rowVersion) &&
    isDate(value.createdAt) &&
    isDate(value.updatedAt) &&
    (value.publishedAt === null || isDate(value.publishedAt)) &&
    (value.archivedAt === null || isDate(value.archivedAt))
  );
}

export function isPublicAnnouncement(value: unknown): value is PublicAnnouncement {
  return (
    isRecord(value) &&
    isUUID(value.id) &&
    typeof value.title === 'string' &&
    nullableString(value.bodyMarkdown) &&
    isDate(value.startsAt) &&
    (value.endsAt === null || isDate(value.endsAt)) &&
    (value.action === null ||
      (isRecord(value.action) &&
        typeof value.action.label === 'string' &&
        typeof value.action.href === 'string' &&
        typeof value.action.external === 'boolean' &&
        safeAnnouncementLink(value.action.href, value.action.external)))
  );
}

export function parseAnnouncementList<T>(
  value: unknown,
  guard: (item: unknown) => item is T,
): AnnouncementList<T> | null {
  return isRecord(value) &&
    Array.isArray(value.announcements) &&
    value.announcements.every(guard) &&
    typeof value.total === 'number' &&
    Number.isSafeInteger(value.total) &&
    value.total >= 0 &&
    typeof value.page === 'number' &&
    Number.isSafeInteger(value.page) &&
    value.page >= 1 &&
    typeof value.pageSize === 'number' &&
    Number.isSafeInteger(value.pageSize) &&
    value.pageSize >= 1 &&
    value.pageSize <= 100
    ? {
        announcements: value.announcements,
        total: value.total,
        page: value.page,
        pageSize: value.pageSize,
      }
    : null;
}

export function parseManagedAnnouncement(value: unknown): ManagedAnnouncement | null {
  return isRecord(value) && isManagedAnnouncement(value.announcement) ? value.announcement : null;
}

export function parseRevisions(value: unknown): readonly AnnouncementRevision[] | null {
  function isRevision(item: unknown): item is AnnouncementRevision {
    return (
      isFields(item) &&
      isRecord(item) &&
      isVersion(item.revision) &&
      isDate(item.publishedAt) &&
      isDate(item.changedAt)
    );
  }
  return isRecord(value) && Array.isArray(value.revisions) && value.revisions.every(isRevision)
    ? value.revisions
    : null;
}

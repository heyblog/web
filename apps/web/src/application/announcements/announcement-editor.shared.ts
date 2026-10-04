import {
  type ActionType,
  type AnnouncementFields,
  type AnnouncementKind,
  type EffectiveStatus,
} from '../../api/announcements/announcements.types.ts';
import type { SessionUser } from '../../api/auth/auth.types.ts';

export interface AnnouncementDraft {
  kind: AnnouncementKind;
  title: string;
  bodyMarkdown: string;
  priority: number | undefined;
  actionType: ActionType;
  actionLabel: string;
  actionTarget: string;
  startsAt: string;
  endsAt: string;
}

export const statusLabels: Readonly<Record<EffectiveStatus, string>> = {
  DRAFT: '草稿',
  SCHEDULED: '待生效',
  ACTIVE: '生效中',
  EXPIRED: '已过期',
  ARCHIVED: '已归档',
};

export function canManageAnnouncements(user: SessionUser): boolean {
  return (
    user.role === 'SYS_ADMIN' ||
    (user.role === 'ADMIN' && user.permissions.includes('announcement.manage'))
  );
}

export function toLocalDateTime(value: string | null): string {
  if (!value) return '';
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return '';
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 19).replace(/:00$/, '');
}

export function fromLocalDateTime(value: string): string | null {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2})?$/.test(value)) return null;
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return null;
  const iso = date.toISOString();
  const canonical = value.length === 19 && value.endsWith(':00') ? value.slice(0, 16) : value;
  return toLocalDateTime(iso) === canonical ? iso : null;
}

export function announcementDraft(fields: AnnouncementFields | null): AnnouncementDraft {
  return {
    kind: fields?.kind ?? 'MAIN',
    title: fields?.title ?? '',
    bodyMarkdown: fields?.bodyMarkdown ?? '',
    priority: fields?.priority ?? 0,
    actionType: fields?.actionType ?? 'NONE',
    actionLabel: fields?.actionLabel ?? '',
    actionTarget: fields?.actionPath ?? fields?.actionExternalUrl ?? '',
    startsAt: toLocalDateTime(fields?.startsAt ?? null),
    endsAt: toLocalDateTime(fields?.endsAt ?? null),
  };
}

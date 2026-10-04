import {
  type AnnouncementFields,
  safeAnnouncementLink,
} from '../../api/announcements/announcements.types.ts';

import {
  type AnnouncementDraft,
  fromLocalDateTime,
  toLocalDateTime,
} from './announcement-editor.shared.ts';

export type AnnouncementField =
  'title' | 'priority' | 'actionLabel' | 'actionTarget' | 'startsAt' | 'endsAt';
export type AnnouncementFieldErrors = Partial<Record<AnnouncementField, string>>;
export type AnnouncementValidation =
  | { readonly kind: 'valid'; readonly payload: AnnouncementFields }
  | { readonly kind: 'invalid'; readonly fieldErrors: AnnouncementFieldErrors };

function draftDate(value: string, previous: string | null): string | null {
  const parsed = fromLocalDateTime(value);
  return parsed === fromLocalDateTime(toLocalDateTime(previous)) ? previous : parsed;
}

export function validateAnnouncementDraft(
  draft: AnnouncementDraft,
  previous: AnnouncementFields | null = null,
): AnnouncementValidation {
  const fieldErrors: AnnouncementFieldErrors = {};
  const title = draft.title.trim();
  if (!title) fieldErrors.title = '请填写公告标题。';
  else if (Array.from(title).length > 256) fieldErrors.title = '标题最多 256 个字符。';
  const priority = draft.kind === 'BANNER' ? 0 : draft.priority;
  if (
    priority === undefined ||
    !Number.isInteger(priority) ||
    priority < -2147483648 ||
    priority > 2147483647
  )
    fieldErrors.priority = '请填写 -2147483648 至 2147483647 之间的整数。';
  const actionLabel = draft.actionLabel.trim();
  const actionTarget = draft.actionTarget.trim();
  if (draft.actionType !== 'NONE') {
    if (!actionLabel) fieldErrors.actionLabel = '请填写链接文字。';
    if (!safeAnnouncementLink(actionTarget, draft.actionType === 'EXTERNAL'))
      fieldErrors.actionTarget =
        draft.actionType === 'INTERNAL'
          ? '请填写以 / 开头的站内路径，不含空格或反斜杠。'
          : '请填写完整的 HTTP(S) 地址，不含账号、密码或空格。';
  }
  const startsAt = draftDate(draft.startsAt, previous?.startsAt ?? null);
  const endsAt = draftDate(draft.endsAt, previous?.endsAt ?? null);
  if (draft.startsAt && startsAt === null) fieldErrors.startsAt = '请填写有效的开始时间。';
  if (draft.endsAt && endsAt === null) fieldErrors.endsAt = '请填写有效的结束时间。';
  if (endsAt !== null) {
    if (!draft.startsAt) fieldErrors.startsAt = '设置结束时间时，请先填写开始时间。';
    else if (startsAt !== null && Date.parse(endsAt) <= Date.parse(startsAt))
      fieldErrors.endsAt = '结束时间必须晚于开始时间。';
  }
  if (Object.keys(fieldErrors).length > 0 || priority === undefined)
    return { kind: 'invalid', fieldErrors };
  return {
    kind: 'valid',
    payload: {
      kind: draft.kind,
      title,
      bodyMarkdown: draft.bodyMarkdown.trim() || null,
      priority,
      actionType: draft.actionType,
      actionLabel: draft.actionType === 'NONE' ? null : actionLabel,
      actionPath: draft.actionType === 'INTERNAL' ? actionTarget : null,
      actionExternalUrl: draft.actionType === 'EXTERNAL' ? actionTarget : null,
      startsAt,
      endsAt,
    },
  };
}

export function validateAnnouncementPublish(
  fields: Pick<AnnouncementFields, 'startsAt' | 'endsAt'>,
  scheduled: boolean,
  now: number,
): AnnouncementFieldErrors {
  const errors: AnnouncementFieldErrors = {};
  if (scheduled && (!fields.startsAt || !(Date.parse(fields.startsAt) > now)))
    errors.startsAt = '定时发布需要设置未来的开始时间，请修改并保存。';
  if (!scheduled && fields.endsAt && !(Date.parse(fields.endsAt) > now))
    errors.endsAt = '立即发布的结束时间必须晚于当前时间，请修改并保存。';
  return errors;
}

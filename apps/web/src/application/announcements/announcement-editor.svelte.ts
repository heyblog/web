import {
  AnnouncementTransportError,
  createAnnouncementTransport,
} from '../../api/announcements/announcements.browser.ts';
import type {
  AnnouncementRevision,
  ManagedAnnouncement,
} from '../../api/announcements/announcements.types.ts';

import type { AnnouncementDraft } from './announcement-editor.shared.ts';
import { announcementDraft } from './announcement-editor.shared.ts';
import {
  type AnnouncementFieldErrors,
  validateAnnouncementDraft,
  validateAnnouncementPublish,
} from './announcement-validation.shared.ts';

export function createAnnouncementEditor(
  initial: ManagedAnnouncement | null,
  initialRevisions: readonly AnnouncementRevision[] | null,
) {
  let announcement = $state(initial);
  const initialDraft = announcementDraft(initial);
  let draft = $state(initialDraft);
  let baseline = $state(JSON.stringify(initialDraft));
  let busy = $state(false);
  let error = $state('');
  let notice = $state('');
  let conflict = $state(false);
  let revisions = $state(initialRevisions);
  let attempted = $state(false);
  let timeErrors = $state<{
    errors: AnnouncementFieldErrors;
    startsAt: string;
    endsAt: string;
  } | null>(null);
  const validation = $derived(validateAnnouncementDraft(draft, announcement));
  const fieldErrors = $derived({
    ...(validation.kind === 'invalid' ? validation.fieldErrors : {}),
    ...(timeErrors && timeErrors.startsAt === draft.startsAt && timeErrors.endsAt === draft.endsAt
      ? timeErrors.errors
      : {}),
  });
  const dirty = $derived(JSON.stringify(draft) !== baseline);
  const transport = createAnnouncementTransport();

  function failure(cause: unknown) {
    if (!(cause instanceof AnnouncementTransportError)) throw cause;
    conflict = cause.status === 409 && cause.code !== 'banner_window_conflict';
    if (cause.status === 401) error = '登录已失效，请重新登录。';
    else if (cause.status === 403) error = '当前账号没有公告管理权限。';
    else if (cause.status === 404) error = '公告已不存在，请返回列表。';
    else if (cause.code === 'banner_window_conflict') {
      error = '横幅发布时间段与其他公告重叠，请调整时间。';
      attempted = true;
      timeErrors = {
        errors: {
          startsAt: '此时间段与其他横幅公告重叠，请调整开始或结束时间。',
          endsAt: '同一时刻只能有一条横幅公告，请调整时间段。',
        },
        startsAt: draft.startsAt,
        endsAt: draft.endsAt,
      };
    } else if (cause.status === 409) error = '公告版本或状态已变化，请重新加载后再操作。';
    else if (cause.status === 422 || cause.status === 400)
      error = '公告内容或时间设置无效，请检查后重试。';
    else error = '操作未完成，请重试。';
  }

  function accept(next: ManagedAnnouncement) {
    announcement = next;
    draft = announcementDraft(next);
    baseline = JSON.stringify(draft);
    conflict = false;
    attempted = false;
    timeErrors = null;
  }

  async function run(operation: () => Promise<void>) {
    if (busy) return;
    busy = true;
    error = '';
    notice = '';
    timeErrors = null;
    try {
      await operation();
    } catch (cause) {
      failure(cause);
    } finally {
      busy = false;
    }
  }

  async function refreshRevisions(id: string) {
    try {
      revisions = await transport.revisions(id);
    } catch (cause) {
      if (!(cause instanceof AnnouncementTransportError)) throw cause;
      revisions = null;
    }
  }

  return {
    get announcement() {
      return announcement;
    },
    get draft() {
      return draft;
    },
    set draft(next: AnnouncementDraft) {
      draft = next;
    },
    get dirty() {
      return dirty;
    },
    get busy() {
      return busy;
    },
    get error() {
      return error;
    },
    get notice() {
      return notice;
    },
    get conflict() {
      return conflict;
    },
    get revisions() {
      return revisions;
    },
    get attempted() {
      return attempted;
    },
    get fieldErrors() {
      return fieldErrors;
    },
    save: () =>
      run(async () => {
        attempted = true;
        const result = validateAnnouncementDraft(draft, announcement);
        if (result.kind === 'invalid') {
          error = '未保存，请检查下方标出的字段。';
          return;
        }
        const fields = result.payload;
        const next = announcement
          ? await transport.update(announcement.id, fields, announcement.rowVersion)
          : await transport.create(fields);
        const created = announcement === null;
        accept(next);
        notice = '已保存。';
        if (created) {
          window.location.assign(`/management/announcements/${next.id}`);
          return;
        }
        await refreshRevisions(next.id);
      }),
    publish: (scheduled: boolean) =>
      run(async () => {
        if (!announcement || dirty || announcement.status !== 'DRAFT') return;
        const errors = validateAnnouncementPublish(announcement, scheduled, Date.now());
        if (Object.keys(errors).length > 0) {
          attempted = true;
          timeErrors = { errors, startsAt: draft.startsAt, endsAt: draft.endsAt };
          error = '未发布，请检查下方标出的时间设置。';
          return;
        }
        const next = await transport.publish(
          announcement.id,
          announcement.rowVersion,
          scheduled ? announcement.startsAt : null,
          announcement.endsAt,
        );
        accept(next);
        notice = scheduled ? '已安排定时发布。' : '已发布。';
      }),
    archive: () =>
      run(async () => {
        if (!announcement || dirty || announcement.status !== 'PUBLISHED') return;
        const next = await transport.archive(announcement.id, announcement.rowVersion);
        accept(next);
        notice = '已归档。';
        await refreshRevisions(next.id);
      }),
    delete: () =>
      run(async () => {
        if (!announcement || announcement.status !== 'DRAFT') return;
        await transport.delete(announcement.id, announcement.rowVersion);
        baseline = JSON.stringify(draft);
        window.location.assign('/management/announcements');
      }),
    reload: () =>
      run(async () => {
        if (!announcement) return;
        const next = await transport.detail(announcement.id);
        accept(next);
        await refreshRevisions(next.id);
        notice = '已加载最新版本。';
      }),
  };
}

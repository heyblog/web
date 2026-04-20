import { Announcements } from '@zhblogs/db';

import { eq } from 'drizzle-orm';
import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';

import { invalidateAnnouncementCache } from '@/application/public/usecase/public-cache.usecase';

import {
  findOverlapConflict,
  isConstraintError,
  normalizeOptionalString,
  normalizePagination,
  readListPagination,
  sendError,
  serializeAnnouncement,
} from './management-announcement.helpers';
import type { AnnouncementBody, AnnouncementListQuery } from './management-announcement.schemas';
import {
  type AnnouncementSavePayload,
  buildAnnouncementValues,
  buildArchiveEligibilityError,
  buildSavePayload,
  createAnnouncement,
  loadAnnouncementById,
  loadAnnouncementRows,
  loadAnnouncementTotal,
  updateAnnouncement,
  validateDraftRevert,
} from './management-announcement.store';

type SaveError = {
  statusCode: number;
  code: string;
  message: string;
};

type AnnouncementRecord = NonNullable<Awaited<ReturnType<typeof loadAnnouncementById>>[number]>;

const buildOverlapError = (overlapId: string): SaveError => ({
  statusCode: 409,
  code: 'ANNOUNCEMENT_WINDOW_CONFLICT',
  message: `The announcement window overlaps with announcement ${overlapId}.`,
});

const persistAnnouncement = async (
  app: FastifyInstance,
  announcementId: string | undefined,
  values: ReturnType<typeof buildAnnouncementValues>,
  actorId: string,
) => {
  if (announcementId) {
    const [updated] = await updateAnnouncement(app, announcementId, values);
    return updated
      ? { data: serializeAnnouncement(updated) }
      : {
          error: {
            statusCode: 404,
            code: 'ANNOUNCEMENT_NOT_FOUND',
            message: 'The target announcement does not exist.',
          } satisfies SaveError,
        };
  }

  const [created] = await createAnnouncement(app, values, actorId);

  if (!created) {
    throw new Error('failed to create announcement');
  }

  return { data: serializeAnnouncement(created) };
};

const resolveOverlapError = async (
  app: FastifyInstance,
  payload: AnnouncementSavePayload,
  currentId: string | undefined,
): Promise<SaveError | null> => {
  const overlapId = await findOverlapConflict(
    app,
    payload.status,
    payload.publishTime,
    payload.expireTime,
    currentId,
  );
  return overlapId ? buildOverlapError(overlapId) : null;
};

const persistAnnouncementWithConstraints = async (
  app: FastifyInstance,
  currentId: string | undefined,
  values: ReturnType<typeof buildAnnouncementValues>,
  actorId: string,
) => {
  try {
    return await persistAnnouncement(app, currentId, values, actorId);
  } catch (error) {
    if (isConstraintError(error, 'announcements_effective_window_exclude')) {
      return {
        error: {
          statusCode: 409,
          code: 'ANNOUNCEMENT_WINDOW_CONFLICT',
          message:
            'The announcement window overlaps with another scheduled or published announcement.',
        } satisfies SaveError,
      };
    }

    if (isConstraintError(error, 'announcements_publish_time_required_check')) {
      return {
        error: {
          statusCode: 400,
          code: 'PUBLISH_TIME_REQUIRED',
          message: 'publish_time is required for scheduled and published announcements.',
        } satisfies SaveError,
      };
    }

    throw error;
  }
};

const isActivePublishedAnnouncement = (existing: AnnouncementRecord, now: Date) => {
  if (existing.status !== 'PUBLISHED') {
    return false;
  }

  if (!existing.publishTime) {
    return false;
  }

  if (existing.publishTime.getTime() > now.getTime()) {
    return false;
  }

  if (existing.expireTime && existing.expireTime.getTime() <= now.getTime()) {
    return false;
  }

  return true;
};

const expireAnnouncement = async (
  app: FastifyInstance,
  announcementId: string,
  actorId: string,
  now: Date,
) => {
  const [updated] = await app.db.write
    .update(Announcements)
    .set({
      status: 'EXPIRED',
      expire_time: now,
      expired_time: now,
      updated_by: actorId,
      updated_time: now,
    })
    .where(eq(Announcements.id, announcementId))
    .returning({
      id: Announcements.id,
      title: Announcements.title,
      content: Announcements.content,
      status: Announcements.status,
      publishTime: Announcements.publish_time,
      expireTime: Announcements.expire_time,
      expiredTime: Announcements.expired_time,
      createdBy: Announcements.created_by,
      updatedBy: Announcements.updated_by,
      createdTime: Announcements.created_time,
      updatedTime: Announcements.updated_time,
    });

  return updated ?? null;
};

const archiveAnnouncement = async (
  app: FastifyInstance,
  announcementId: string,
  actorId: string,
  now: Date,
) => {
  const [existing] = await loadAnnouncementById(app, announcementId);

  if (!existing) {
    return {
      error: {
        statusCode: 404,
        code: 'ANNOUNCEMENT_NOT_FOUND',
        message: 'The target announcement does not exist.',
      } satisfies SaveError,
    };
  }

  if (!isActivePublishedAnnouncement(existing, now)) {
    return { error: buildArchiveEligibilityError() };
  }

  const updated = await expireAnnouncement(app, announcementId, actorId, now);

  if (!updated) {
    return {
      error: {
        statusCode: 404,
        code: 'ANNOUNCEMENT_NOT_FOUND',
        message: 'The target announcement does not exist.',
      } satisfies SaveError,
    };
  }

  return { data: serializeAnnouncement(updated) };
};

export const buildListAnnouncementsHandler =
  (app: FastifyInstance) => async (request: FastifyRequest) => {
    const { page, pageSize } = readListPagination(request.query as AnnouncementListQuery);
    const totalItems = await loadAnnouncementTotal(app);
    const pagination = normalizePagination(page, pageSize, totalItems);
    const rows = await loadAnnouncementRows(app, pagination);

    return {
      ok: true,
      data: {
        items: rows.map(serializeAnnouncement),
        pagination,
      },
    };
  };

export const buildSaveAnnouncementHandler =
  (app: FastifyInstance) =>
  async (request: FastifyRequest, reply: FastifyReply, actorId: string) => {
    const body = (request.body ?? {}) as AnnouncementBody;
    const now = new Date();
    const result = buildSavePayload(body, now);

    if ('error' in result) {
      return sendError(reply, result.error.statusCode, result.error.code, result.error.message);
    }

    const currentId = normalizeOptionalString(body.id) ?? undefined;
    const draftError = await validateDraftRevert(app, currentId, result.payload.status);

    if (draftError) {
      return sendError(reply, draftError.statusCode, draftError.code, draftError.message);
    }

    const overlapError = await resolveOverlapError(app, result.payload, currentId);

    if (overlapError) {
      return sendError(reply, overlapError.statusCode, overlapError.code, overlapError.message);
    }

    const values = buildAnnouncementValues(result.payload, actorId, now);

    const persisted = await persistAnnouncementWithConstraints(app, currentId, values, actorId);

    if (persisted.error) {
      return sendError(
        reply,
        persisted.error.statusCode,
        persisted.error.code,
        persisted.error.message,
      );
    }

    await invalidateAnnouncementCache(app);

    return {
      ok: true,
      data: persisted.data,
    };
  };

export const buildArchiveAnnouncementHandler =
  (app: FastifyInstance) =>
  async (request: FastifyRequest, reply: FastifyReply, actorId: string) => {
    const now = new Date();
    const { id } = request.params as { id: string };
    const archived = await archiveAnnouncement(app, id, actorId, now);

    if (archived.error) {
      return sendError(
        reply,
        archived.error.statusCode,
        archived.error.code,
        archived.error.message,
      );
    }

    await invalidateAnnouncementCache(app);

    return {
      ok: true,
      data: archived.data,
    };
  };

export const buildDeleteAnnouncementHandler =
  (app: FastifyInstance) => async (request: FastifyRequest, reply: FastifyReply) => {
    const [existing] = await app.db.read
      .select({
        id: Announcements.id,
        status: Announcements.status,
      })
      .from(Announcements)
      .where(eq(Announcements.id, (request.params as { id: string }).id))
      .limit(1);

    if (!existing) {
      return sendError(
        reply,
        404,
        'ANNOUNCEMENT_NOT_FOUND',
        'The target announcement does not exist.',
      );
    }

    if (existing.status !== 'DRAFT') {
      return sendError(
        reply,
        409,
        'ANNOUNCEMENT_DELETE_FORBIDDEN',
        'Only draft announcements can be deleted.',
      );
    }

    await app.db.write.delete(Announcements).where(eq(Announcements.id, existing.id));
    await invalidateAnnouncementCache(app);

    return {
      ok: true,
    };
  };

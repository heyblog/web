import { SiteAudits, type SiteAuditSnapshot, Sites } from '@zhblogs/db';

import { eq } from 'drizzle-orm';
import type { FastifyInstance } from 'fastify';

import { invalidatePublicSiteCache } from '@/application/public/usecase/public-cache.usecase';
import {
  syncSiteArchitecture,
  syncSiteTags,
} from '@/application/sites/usecase/site-architecture-sync.usecase';
import { listManualReadOnlySiteManagementFieldChanges } from '@/domain/sites/service/site-management-access.service';
import { normalizeManagementSiteSnapshot } from '@/domain/sites/service/site-management-snapshot.service';
import {
  buildSelectedTagIds,
  buildSnapshotDiff,
} from '@/domain/sites/service/site-snapshot.service';
import {
  ensureNoSiteIdentifierConflict,
  ensureTagIdsExist,
  ensureTechnologyIdsExist,
  loadCurrentSiteSnapshot,
} from '@/infrastructure/sites/db/site.repository';

import {
  normalizeOptionalString,
  requireManagementPermission,
  sendManagementError,
} from './management-route.shared';

export function registerManagementSiteUpdateRoute(app: FastifyInstance): void {
  app.put<{ Params: { siteId: string } }>(
    '/api/management/sites/:siteId',
    {
      preHandler: requireManagementPermission('site.manage'),
    },
    async (request, reply) => {
      const actor = await app.auth.getCurrentUser(request);
      const body = request.body as { snapshot?: SiteAuditSnapshot; comment?: string | null };

      if (!body.snapshot) {
        return sendManagementError(reply, 400, 'INVALID_BODY', 'snapshot is required.');
      }

      const currentSnapshot = await loadCurrentSiteSnapshot(app, request.params.siteId);

      if (!currentSnapshot) {
        return sendManagementError(reply, 404, 'SITE_NOT_FOUND', 'The target site does not exist.');
      }

      const normalizedSnapshot = normalizeManagementSiteSnapshot(body.snapshot);
      const forbiddenFields = listManualReadOnlySiteManagementFieldChanges(
        currentSnapshot,
        normalizedSnapshot,
      );

      if (forbiddenFields.length > 0) {
        return sendManagementError(
          reply,
          403,
          'SITE_FIELD_FORBIDDEN',
          `Read-only site fields cannot be modified manually: ${forbiddenFields.join(', ')}.`,
        );
      }

      const conflictFields = await ensureNoSiteIdentifierConflict(
        app,
        normalizedSnapshot,
        request.params.siteId,
      );

      if (conflictFields) {
        return sendManagementError(
          reply,
          409,
          'SITE_CONFLICT',
          `A site already exists with the same ${conflictFields.join(', ')}.`,
        );
      }

      if (
        !(await ensureTagIdsExist(
          app,
          buildSelectedTagIds(normalizedSnapshot.main_tag, normalizedSnapshot.sub_tags),
        ))
      ) {
        return sendManagementError(
          reply,
          400,
          'INVALID_TAGS',
          'One or more selected tags do not exist.',
        );
      }

      if (!(await ensureTechnologyIdsExist(app, normalizedSnapshot.architecture))) {
        return sendManagementError(
          reply,
          400,
          'INVALID_TECHNOLOGIES',
          'One or more selected technologies do not exist.',
        );
      }

      await app.db.write
        .update(Sites)
        .set({
          bid: normalizedSnapshot.bid ?? null,
          name: normalizedSnapshot.name ?? '',
          url: normalizedSnapshot.url ?? '',
          sign: normalizedSnapshot.sign ?? '',
          icon_base64: normalizedSnapshot.icon_base64 ?? null,
          feed: normalizedSnapshot.feed ?? [],
          from: normalizedSnapshot.from ?? ['WEB_SUBMIT'],
          classification_status: normalizedSnapshot.classification_status ?? 'NEEDS_REVIEW',
          sitemap: normalizedSnapshot.sitemap ?? null,
          link_page: normalizedSnapshot.link_page ?? null,
          access_scope: normalizedSnapshot.access_scope ?? 'ALL',
          status: normalizedSnapshot.status ?? 'OK',
          is_show: normalizedSnapshot.is_show ?? true,
          recommend: normalizedSnapshot.recommend ?? false,
          reason: normalizedSnapshot.reason ?? null,
          update_time: new Date(),
        })
        .where(eq(Sites.id, request.params.siteId));

      await syncSiteTags(app, request.params.siteId, normalizedSnapshot);
      await syncSiteArchitecture(app, request.params.siteId, normalizedSnapshot);
      await invalidatePublicSiteCache(app);
      const appliedSnapshot = await loadCurrentSiteSnapshot(app, request.params.siteId);

      if (!appliedSnapshot) {
        return sendManagementError(
          reply,
          500,
          'SITE_SNAPSHOT_ERROR',
          'Failed to reload managed site snapshot.',
        );
      }

      const reviewerComment = normalizeOptionalString(body.comment);

      await app.db.write.insert(SiteAudits).values({
        site_id: request.params.siteId,
        action: 'UPDATE',
        status: 'APPROVED',
        current_snapshot: currentSnapshot,
        proposed_snapshot: appliedSnapshot,
        diff: buildSnapshotDiff(currentSnapshot, appliedSnapshot),
        submit_reason: reviewerComment,
        reviewer_comment: reviewerComment,
        reviewed_by: actor.id,
        submitter_name: actor.nickname,
        submitter_email: actor.email,
        notify_by_email: false,
        reviewed_time: new Date(),
      });

      return {
        ok: true,
        data: {
          site_id: request.params.siteId,
        },
      };
    },
  );
}

import { SiteAudits, type SiteAuditSnapshot } from '@zhblogs/db';

import { eq } from 'drizzle-orm';
import type { FastifyInstance, FastifyReply, preHandlerHookHandler } from 'fastify';

import { listManualReadOnlySiteManagementFieldChanges } from '@/domain/sites/service/site-management-access.service';
import { normalizeManagementSiteSnapshot } from '@/domain/sites/service/site-management-snapshot.service';
import { buildSnapshotDiff } from '@/domain/sites/service/site-snapshot.service';
import type { AuditReviewInput } from '@/presentation/sites/dto/site-request.dto';

type ParseResult<T> = { success: true; data: T } | { success: false };

type SafeParser<T> = {
  safeParse: (input: unknown) => ParseResult<T>;
};

type ErrorResponder = (
  reply: FastifyReply,
  statusCode: number,
  code: string,
  message: string,
  fields?: string[],
  details?: Record<string, unknown>,
) => unknown;

type UpdatedAudit = {
  id: string;
  action: string;
  status: string;
  site_id: string | null;
  notify_by_email: boolean;
  submitter_email: string | null;
  reviewer_comment: string | null;
  current_snapshot: unknown;
  proposed_snapshot: unknown;
};

type ReviewRouteDeps = {
  requireAdminReviewer: preHandlerHookHandler;
  auditIdParamJsonSchema: unknown;
  submissionResultSchema: unknown;
  errorResponseSchema: unknown;
  siteIdParamSchema: SafeParser<{ site_id: string | null }>;
  auditReviewSchema: SafeParser<AuditReviewInput>;
  sendApiError: ErrorResponder;
  applyApprovedAudit: (
    app: FastifyInstance,
    audit: {
      id: string;
      action: 'CREATE' | 'UPDATE' | 'DELETE' | 'RESTORE';
      site_id: string | null;
      submit_reason: string | null;
      reviewer_comment: string | null;
      current_snapshot: SiteAuditSnapshot | null;
      proposed_snapshot: SiteAuditSnapshot | null;
      snapshot_override?: SiteAuditSnapshot | null;
    },
  ) => Promise<string | null>;
  finalizeAuditReview: (
    app: FastifyInstance,
    auditId: string,
    decision: 'APPROVED' | 'REJECTED',
    reviewerComment: string | null,
    reviewerId: string,
    siteId: string | null,
  ) => Promise<UpdatedAudit | null>;
  loadCurrentSiteSnapshot: (
    app: FastifyInstance,
    siteId: string,
  ) => Promise<SiteAuditSnapshot | null>;
  materializeSiteAuditSnapshot: (
    app: FastifyInstance,
    snapshot: SiteAuditSnapshot,
  ) => Promise<SiteAuditSnapshot>;
  enqueueFeedDetectionJobs: (
    app: FastifyInstance,
    siteId: string,
    snapshot: SiteAuditSnapshot | null | undefined,
    triggerKey: string,
    action: 'CREATE' | 'UPDATE' | 'RESTORE',
  ) => Promise<void>;
  canSendSubmissionDecisionMail: (config: FastifyInstance['config']) => boolean;
  sendSubmissionDecisionMail: (
    config: FastifyInstance['config'],
    payload: {
      recipient: string;
      auditId: string;
      siteName: string;
      action: string;
      status: 'APPROVED' | 'REJECTED';
      reviewerComment: string | null;
    },
  ) => Promise<void>;
  resolveAuditSiteName: (
    proposed: SiteAuditSnapshot | null | undefined,
    current: SiteAuditSnapshot | null | undefined,
  ) => string | null;
};

export function registerAdminAuditReviewRoute(app: FastifyInstance, deps: ReviewRouteDeps): void {
  app.post<{ Params: { auditId: string } }>(
    '/api/management/site-audits/:auditId/review',
    {
      preHandler: deps.requireAdminReviewer,
      schema: {
        params: deps.auditIdParamJsonSchema,
        response: {
          200: deps.submissionResultSchema,
          400: deps.errorResponseSchema,
          403: deps.errorResponseSchema,
          404: deps.errorResponseSchema,
          409: deps.errorResponseSchema,
          503: deps.errorResponseSchema,
        },
      },
      config: {
        rateLimit: {
          max: 30,
          timeWindow: '1 minute',
        },
      },
    },
    async (request, reply) => {
      const actor = await app.auth.getCurrentUser(request);
      const parsedAuditId = deps.siteIdParamSchema.safeParse({
        site_id: request.params.auditId,
      });

      if (!parsedAuditId.success || parsedAuditId.data.site_id === null) {
        return deps.sendApiError(reply, 400, 'INVALID_AUDIT_ID', 'auditId must be a valid UUID.');
      }

      const parsedBody = deps.auditReviewSchema.safeParse(request.body);

      if (!parsedBody.success) {
        return deps.sendApiError(
          reply,
          400,
          'INVALID_BODY',
          'Request body is invalid for audit review.',
        );
      }

      const [audit] = await app.db.read
        .select({
          id: SiteAudits.id,
          action: SiteAudits.action,
          status: SiteAudits.status,
          site_id: SiteAudits.site_id,
          submit_reason: SiteAudits.submit_reason,
          submitter_email: SiteAudits.submitter_email,
          notify_by_email: SiteAudits.notify_by_email,
          current_snapshot: SiteAudits.current_snapshot,
          proposed_snapshot: SiteAudits.proposed_snapshot,
        })
        .from(SiteAudits)
        .where(eq(SiteAudits.id, parsedAuditId.data.site_id))
        .limit(1);

      if (!audit) {
        return deps.sendApiError(reply, 404, 'AUDIT_NOT_FOUND', 'The target audit does not exist.');
      }

      if (audit.status !== 'PENDING') {
        return deps.sendApiError(
          reply,
          409,
          'AUDIT_ALREADY_REVIEWED',
          'The target audit has already been reviewed.',
        );
      }

      const reviewerComment = parsedBody.data.reviewer_comment?.trim() ?? null;
      const validatedSnapshotOverride = parsedBody.data.snapshot_override as
        | SiteAuditSnapshot
        | null
        | undefined;
      const snapshotOverride = validatedSnapshotOverride
        ? normalizeManagementSiteSnapshot(validatedSnapshotOverride)
        : null;

      try {
        let siteId = audit.site_id;
        const currentSnapshot = audit.current_snapshot as SiteAuditSnapshot | null;
        const proposedSnapshot = audit.proposed_snapshot as SiteAuditSnapshot | null;
        const allowsSnapshotOverride = audit.action === 'CREATE' || audit.action === 'UPDATE';

        if (
          parsedBody.data.decision === 'APPROVED' &&
          snapshotOverride &&
          !allowsSnapshotOverride
        ) {
          return deps.sendApiError(
            reply,
            400,
            'INVALID_BODY',
            'snapshot_override is only allowed for CREATE or UPDATE audits.',
          );
        }

        if (parsedBody.data.decision === 'APPROVED' && snapshotOverride) {
          const forbiddenFields = listManualReadOnlySiteManagementFieldChanges(
            proposedSnapshot ?? currentSnapshot,
            snapshotOverride,
          );

          if (forbiddenFields.length > 0) {
            return deps.sendApiError(
              reply,
              403,
              'SITE_FIELD_FORBIDDEN',
              `Read-only site fields cannot be modified manually: ${forbiddenFields.join(', ')}.`,
            );
          }
        }

        if (parsedBody.data.decision === 'APPROVED') {
          siteId = await deps.applyApprovedAudit(app, {
            id: audit.id,
            action: audit.action as 'CREATE' | 'UPDATE' | 'DELETE' | 'RESTORE',
            site_id: audit.site_id,
            submit_reason: audit.submit_reason,
            reviewer_comment: reviewerComment,
            current_snapshot: currentSnapshot,
            proposed_snapshot: proposedSnapshot,
            snapshot_override: snapshotOverride,
          });
        }

        const updatedAudit = await deps.finalizeAuditReview(
          app,
          audit.id,
          parsedBody.data.decision,
          reviewerComment,
          actor.id,
          siteId ?? null,
        );

        if (!updatedAudit) {
          throw new Error('failed to update audit review result');
        }

        const appliedSnapshot =
          parsedBody.data.decision === 'APPROVED' && updatedAudit.site_id
            ? await deps.loadCurrentSiteSnapshot(app, updatedAudit.site_id)
            : null;
        const approvedSiteId = updatedAudit.site_id;

        if (parsedBody.data.decision === 'APPROVED' && appliedSnapshot && approvedSiteId) {
          const materializedSubmittedSnapshot = proposedSnapshot
            ? await deps.materializeSiteAuditSnapshot(app, proposedSnapshot)
            : null;
          const finalDiff = buildSnapshotDiff(currentSnapshot, appliedSnapshot);
          const overrideDiff =
            snapshotOverride && materializedSubmittedSnapshot
              ? buildSnapshotDiff(materializedSubmittedSnapshot, appliedSnapshot)
              : [];

          await app.db.write
            .update(SiteAudits)
            .set({
              current_snapshot: currentSnapshot,
              proposed_snapshot: materializedSubmittedSnapshot ?? proposedSnapshot,
              diff: finalDiff,
              review_override_snapshot: overrideDiff.length > 0 ? appliedSnapshot : null,
              review_override_diff: overrideDiff.length > 0 ? overrideDiff : null,
            })
            .where(eq(SiteAudits.id, updatedAudit.id));

          const isLifecycleAction =
            updatedAudit.action === 'CREATE' ||
            updatedAudit.action === 'UPDATE' ||
            updatedAudit.action === 'RESTORE';

          if (isLifecycleAction) {
            const lifecycleAction = updatedAudit.action as 'CREATE' | 'UPDATE' | 'RESTORE';
            await deps.enqueueFeedDetectionJobs(
              app,
              approvedSiteId,
              appliedSnapshot,
              `site-audit:${updatedAudit.id}:${updatedAudit.action}`,
              lifecycleAction,
            );
          }
        }

        if (
          updatedAudit.notify_by_email &&
          updatedAudit.submitter_email &&
          deps.canSendSubmissionDecisionMail(app.config)
        ) {
          const siteName =
            deps.resolveAuditSiteName(
              appliedSnapshot ??
                (updatedAudit.proposed_snapshot as SiteAuditSnapshot | null | undefined),
              currentSnapshot ??
                (updatedAudit.current_snapshot as SiteAuditSnapshot | null | undefined),
            ) ?? '未命名站点';

          try {
            await deps.sendSubmissionDecisionMail(app.config, {
              recipient: updatedAudit.submitter_email,
              auditId: updatedAudit.id,
              siteName,
              action: updatedAudit.action,
              status: parsedBody.data.decision,
              reviewerComment: updatedAudit.reviewer_comment ?? null,
            });
          } catch (error) {
            app.log.error(
              {
                error,
                auditId: updatedAudit.id,
                recipient: updatedAudit.submitter_email,
                siteName,
                action: updatedAudit.action,
                decision: parsedBody.data.decision,
              },
              'failed to send submission decision mail',
            );
          }
        }

        return {
          ok: true,
          data: {
            audit_id: updatedAudit.id,
            action: updatedAudit.action,
            status: updatedAudit.status,
            site_id: updatedAudit.site_id,
          },
        };
      } catch (error) {
        app.log.error({ error, auditId: audit.id }, 'failed to review site audit');

        if (error instanceof Error && error.message.startsWith('site conflict:')) {
          return deps.sendApiError(
            reply,
            409,
            'SITE_CONFLICT',
            'A site with the same unique identifier already exists.',
          );
        }

        return deps.sendApiError(
          reply,
          503,
          'DEPENDENCY_ERROR',
          'Unable to review the site audit right now.',
        );
      }
    },
  );
}

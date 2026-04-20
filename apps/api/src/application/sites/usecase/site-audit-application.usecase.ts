import { SiteAudits, type SiteAuditSnapshot, Sites } from '@zhblogs/db';

import { eq } from 'drizzle-orm';
import type { FastifyInstance } from 'fastify';

import { enqueueJobs } from '@/application/jobs/usecase';
import { invalidatePublicSiteCache } from '@/application/public/usecase/public-cache.usecase';
import { normalizeManagementSiteSnapshot } from '@/domain/sites/service/site-management-snapshot.service';

type AuditAction = 'CREATE' | 'UPDATE' | 'DELETE' | 'RESTORE';

type AuditPayload = {
  id: string;
  action: AuditAction;
  site_id: string | null;
  submit_reason: string | null;
  reviewer_comment: string | null;
  proposed_snapshot: SiteAuditSnapshot | null;
  current_snapshot: SiteAuditSnapshot | null;
  snapshot_override?: SiteAuditSnapshot | null;
};

type ApplyDeps = {
  ensureNoSiteIdentifierConflict: (
    app: FastifyInstance,
    snapshot: SiteAuditSnapshot,
    ignoreSiteId?: string,
  ) => Promise<Array<'url' | 'bid' | 'name'> | null>;
  syncSiteTags: (
    app: FastifyInstance,
    siteId: string,
    snapshot: SiteAuditSnapshot,
  ) => Promise<void>;
  syncSiteArchitecture: (
    app: FastifyInstance,
    siteId: string,
    snapshot: SiteAuditSnapshot,
  ) => Promise<void>;
};

const buildSoftDeleteReason = (submitReason: string, reviewerComment?: string | null): string => {
  const parts = [submitReason.trim()];

  if (reviewerComment?.trim()) {
    parts.push(`审核备注：${reviewerComment.trim()}`);
  }

  return parts.join(' ｜ ');
};

export async function finalizeAuditReview(
  app: FastifyInstance,
  auditId: string,
  status: 'APPROVED' | 'REJECTED',
  reviewerComment: string | null,
  reviewerId: string,
  siteId: string | null,
) {
  const [updatedAudit] = await app.db.write
    .update(SiteAudits)
    .set({
      status,
      reviewer_comment: reviewerComment,
      reviewed_by: reviewerId,
      reviewed_time: new Date(),
      ...(siteId ? { site_id: siteId } : {}),
    })
    .where(eq(SiteAudits.id, auditId))
    .returning({
      id: SiteAudits.id,
      action: SiteAudits.action,
      status: SiteAudits.status,
      site_id: SiteAudits.site_id,
      submitter_email: SiteAudits.submitter_email,
      notify_by_email: SiteAudits.notify_by_email,
      reviewer_comment: SiteAudits.reviewer_comment,
      proposed_snapshot: SiteAudits.proposed_snapshot,
      current_snapshot: SiteAudits.current_snapshot,
    });

  return updatedAudit ?? null;
}

export async function applyApprovedAudit(
  app: FastifyInstance,
  audit: AuditPayload,
  deps: ApplyDeps,
) {
  const snapshotSource = (audit.snapshot_override ??
    audit.proposed_snapshot ??
    audit.current_snapshot) as SiteAuditSnapshot | null;
  const snapshot = snapshotSource ? normalizeManagementSiteSnapshot(snapshotSource) : null;

  if (!snapshot) {
    throw new Error('audit snapshot is missing');
  }

  if (audit.action === 'CREATE') {
    const conflictFields = await deps.ensureNoSiteIdentifierConflict(app, snapshot);

    if (conflictFields) {
      throw new Error(`site conflict: ${conflictFields.join(',')}`);
    }

    const [createdSite] = await app.db.write
      .insert(Sites)
      .values({
        bid: snapshot.bid ?? null,
        name: snapshot.name ?? '',
        url: snapshot.url ?? '',
        sign: snapshot.sign ?? '',
        icon_base64: snapshot.icon_base64 ?? null,
        feed: snapshot.feed ?? [],
        from: snapshot.from ?? ['WEB_SUBMIT'],
        classification_status: snapshot.main_tag?.tag_id ? 'COMPLETE' : 'NEEDS_REVIEW',
        sitemap: snapshot.sitemap ?? null,
        link_page: snapshot.link_page ?? null,
        access_scope: snapshot.access_scope ?? 'ALL',
        status: snapshot.status ?? 'OK',
        is_show: true,
        recommend: snapshot.recommend ?? false,
        reason: null,
      })
      .returning({
        id: Sites.id,
      });

    if (!createdSite) {
      throw new Error('failed to create site');
    }

    await deps.syncSiteTags(app, createdSite.id, snapshot);
    await deps.syncSiteArchitecture(app, createdSite.id, snapshot);
    await invalidatePublicSiteCache(app);

    return createdSite.id;
  }

  if (!audit.site_id) {
    throw new Error('audit site_id is missing');
  }

  if (audit.action === 'UPDATE') {
    const conflictFields = await deps.ensureNoSiteIdentifierConflict(app, snapshot, audit.site_id);

    if (conflictFields) {
      throw new Error(`site conflict: ${conflictFields.join(',')}`);
    }

    await app.db.write
      .update(Sites)
      .set({
        bid: snapshot.bid ?? null,
        name: snapshot.name ?? '',
        url: snapshot.url ?? '',
        sign: snapshot.sign ?? '',
        icon_base64: snapshot.icon_base64 ?? null,
        feed: snapshot.feed ?? [],
        from: snapshot.from ?? ['WEB_SUBMIT'],
        classification_status: snapshot.main_tag?.tag_id ? 'COMPLETE' : 'NEEDS_REVIEW',
        sitemap: snapshot.sitemap ?? null,
        link_page: snapshot.link_page ?? null,
        access_scope: snapshot.access_scope ?? 'ALL',
        status: snapshot.status ?? 'OK',
        is_show: snapshot.is_show ?? true,
        recommend: snapshot.recommend ?? false,
        reason: snapshot.reason ?? null,
        update_time: new Date(),
      })
      .where(eq(Sites.id, audit.site_id));

    await deps.syncSiteTags(app, audit.site_id, snapshot);
    await deps.syncSiteArchitecture(app, audit.site_id, snapshot);
    await invalidatePublicSiteCache(app);

    return audit.site_id;
  }

  if (audit.action === 'RESTORE') {
    await app.db.write
      .update(Sites)
      .set({
        is_show: true,
        reason: null,
        update_time: new Date(),
      })
      .where(eq(Sites.id, audit.site_id));
    await invalidatePublicSiteCache(app);

    return audit.site_id;
  }

  await app.db.write
    .update(Sites)
    .set({
      is_show: false,
      reason: buildSoftDeleteReason(audit.submit_reason ?? '', audit.reviewer_comment),
      update_time: new Date(),
    })
    .where(eq(Sites.id, audit.site_id));
  await invalidatePublicSiteCache(app);

  return audit.site_id;
}

export async function enqueueFeedDetectionJobs(
  app: FastifyInstance,
  siteId: string,
  snapshot: SiteAuditSnapshot | null | undefined,
  triggerKey: string,
  normalizeSubmittedFeeds: (
    feed: SiteAuditSnapshot['feed'] | null | undefined,
  ) => Array<{ url: string }>,
  action: 'CREATE' | 'UPDATE' | 'RESTORE' = 'UPDATE',
) {
  const feed = normalizeSubmittedFeeds(snapshot?.feed);

  const jobs = [
    {
      task_type: 'SITE_CHECK' as const,
      trigger_source: 'EVENT' as const,
      payload: {
        target: {
          kind: 'SITE',
          site_id: siteId,
        },
        options: {
          source: 'site-audit',
          action,
          run_content_validation: true,
        },
      },
    },
  ];

  const rssJobs =
    feed.length > 0
      ? feed.map((item, index) => ({
          task_type: 'RSS_FETCH' as const,
          trigger_source: 'EVENT' as const,
          payload: {
            target: {
              kind: 'SITE',
              site_id: siteId,
            },
            options: {
              source: 'site-audit',
              action,
              first_fetch: true,
              feed_mode: 'DEFAULT_ONLY',
              feed_urls: [item.url],
              feed_index: index,
            },
          },
        }))
      : [
          {
            task_type: 'RSS_FETCH' as const,
            trigger_source: 'EVENT' as const,
            payload: {
              target: {
                kind: 'SITE',
                site_id: siteId,
              },
              options: {
                source: 'site-audit',
                action,
                first_fetch: true,
                feed_mode: 'DEFAULT_ONLY',
              },
            },
          },
        ];

  await enqueueJobs(app, [...jobs, ...rssJobs]);
}

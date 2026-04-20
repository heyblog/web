import {
  type SiteAuditDiffItem,
  SiteAudits,
  type SiteAuditSnapshot,
  SiteTags,
  SiteWarningTags,
  TagDefinitions,
  TechnologyCatalogs,
} from '@zhblogs/db';

import { asc, eq } from 'drizzle-orm';
import type { FastifyInstance } from 'fastify';

import { invalidatePublicSiteCache } from '@/application/public/usecase/public-cache.usecase';
import { buildSnapshotDiff } from '@/domain/sites/service/site-snapshot.service';

import {
  normalizeOptionalString,
  requireManagementPermission,
  sendManagementError,
} from './management-route.shared';
import {
  rewriteDiffValueForTagMerge,
  rewriteSnapshotForTagMerge,
} from './management-taxonomy.shared';

export function registerManagementTaxonomyTagRoutes(app: FastifyInstance): void {
  app.get(
    '/api/management/taxonomy/overview',
    {
      preHandler: requireManagementPermission('taxonomy.manage'),
    },
    async () => {
      const [tags, technologies] = await Promise.all([
        app.db.read
          .select()
          .from(TagDefinitions)
          .orderBy(asc(TagDefinitions.tag_type), asc(TagDefinitions.name)),
        app.db.read
          .select()
          .from(TechnologyCatalogs)
          .orderBy(asc(TechnologyCatalogs.technology_type), asc(TechnologyCatalogs.name)),
      ]);

      return {
        ok: true,
        data: {
          tags: tags.map((row) => ({
            ...row,
            merged_time: row.merged_time?.toISOString() ?? null,
            created_time: row.created_time.toISOString(),
            updated_time: row.updated_time.toISOString(),
          })),
          technologies: technologies.map((row) => ({
            ...row,
            created_time: row.created_time.toISOString(),
            updated_time: row.updated_time.toISOString(),
          })),
        },
      };
    },
  );

  app.post(
    '/api/management/taxonomy/tags',
    {
      preHandler: requireManagementPermission('taxonomy.manage'),
    },
    async (request, reply) => {
      const body = request.body as {
        id?: string;
        name?: string;
        machine_key?: string | null;
        tag_type?: 'MAIN' | 'SUB' | 'WARNING';
        description?: string | null;
        is_enabled?: boolean;
      };

      if (!body.name?.trim() || !body.tag_type) {
        return sendManagementError(reply, 400, 'INVALID_BODY', 'name and tag_type are required.');
      }

      if (body.tag_type === 'WARNING' && !normalizeOptionalString(body.machine_key)) {
        return sendManagementError(reply, 400, 'INVALID_BODY', 'WARNING tags require machine_key.');
      }

      const values = {
        name: body.name.trim(),
        machine_key: normalizeOptionalString(body.machine_key),
        tag_type: body.tag_type,
        description: normalizeOptionalString(body.description),
        is_enabled: body.is_enabled ?? true,
      };

      const rows = body.id
        ? await app.db.write
            .update(TagDefinitions)
            .set(values)
            .where(eq(TagDefinitions.id, body.id))
            .returning()
        : await app.db.write.insert(TagDefinitions).values(values).returning();
      const row = Array.isArray(rows) ? (rows[0] ?? null) : null;
      await invalidatePublicSiteCache(app);

      return {
        ok: true,
        data: row,
      };
    },
  );

  app.post<{ Params: { tagId: string } }>(
    '/api/management/taxonomy/tags/:tagId/merge',
    {
      preHandler: requireManagementPermission('taxonomy.manage'),
    },
    async (request, reply) => {
      const actor = await app.auth.getCurrentUser(request);
      const body = request.body as { target_tag_id?: string };

      if (!body.target_tag_id?.trim()) {
        return sendManagementError(reply, 400, 'INVALID_BODY', 'target_tag_id is required.');
      }

      if (body.target_tag_id === request.params.tagId) {
        return sendManagementError(
          reply,
          400,
          'INVALID_BODY',
          'source and target tags must be different.',
        );
      }

      const [sourceTag] = await app.db.read
        .select()
        .from(TagDefinitions)
        .where(eq(TagDefinitions.id, request.params.tagId))
        .limit(1);
      const [targetTag] = await app.db.read
        .select()
        .from(TagDefinitions)
        .where(eq(TagDefinitions.id, body.target_tag_id))
        .limit(1);

      if (!sourceTag || !targetTag) {
        return sendManagementError(
          reply,
          404,
          'TAG_NOT_FOUND',
          'Source or target tag does not exist.',
        );
      }

      if (!sourceTag.is_enabled || !targetTag.is_enabled) {
        return sendManagementError(reply, 409, 'TAG_DISABLED', 'Merged tags must both be enabled.');
      }

      if (sourceTag.tag_type !== targetTag.tag_type) {
        return sendManagementError(
          reply,
          409,
          'TAG_TYPE_MISMATCH',
          'Only tags of the same type can be merged.',
        );
      }

      let migratedSiteRefs = 0;
      let migratedWarningRefs = 0;
      let rewrittenAudits = 0;

      if (sourceTag.tag_type === 'WARNING') {
        const warningRows = await app.db.read
          .select()
          .from(SiteWarningTags)
          .where(eq(SiteWarningTags.tag_id, sourceTag.id));

        for (const row of warningRows) {
          await app.db.write
            .insert(SiteWarningTags)
            .values({
              site_id: row.site_id,
              tag_id: targetTag.id,
              source: row.source,
              source_site_audit_id: row.source_site_audit_id,
              source_article_feedback_audit_id: row.source_article_feedback_audit_id,
              note: row.note,
              created_by: row.created_by,
            })
            .onConflictDoUpdate({
              target: [SiteWarningTags.site_id, SiteWarningTags.tag_id, SiteWarningTags.source],
              set: {
                note: row.note,
                updated_time: new Date(),
              },
            });
        }

        if (warningRows.length > 0) {
          await app.db.write
            .delete(SiteWarningTags)
            .where(eq(SiteWarningTags.tag_id, sourceTag.id));
        }

        migratedWarningRefs = warningRows.length;
      } else {
        const siteRows = await app.db.read
          .select({ site_id: SiteTags.site_id })
          .from(SiteTags)
          .where(eq(SiteTags.tag_id, sourceTag.id));

        if (siteRows.length > 0) {
          await app.db.write
            .insert(SiteTags)
            .values(
              siteRows.map((row) => ({
                site_id: row.site_id,
                tag_id: targetTag.id,
              })),
            )
            .onConflictDoNothing();

          await app.db.write.delete(SiteTags).where(eq(SiteTags.tag_id, sourceTag.id));
        }

        migratedSiteRefs = siteRows.length;
      }

      const pendingAudits = await app.db.read
        .select({
          id: SiteAudits.id,
          action: SiteAudits.action,
          diff: SiteAudits.diff,
          current_snapshot: SiteAudits.current_snapshot,
          proposed_snapshot: SiteAudits.proposed_snapshot,
        })
        .from(SiteAudits)
        .where(eq(SiteAudits.status, 'PENDING'));

      for (const audit of pendingAudits) {
        const currentSnapshot = rewriteSnapshotForTagMerge(
          (audit.current_snapshot as SiteAuditSnapshot | null) ?? null,
          sourceTag.id,
          targetTag.id,
          sourceTag.tag_type as 'MAIN' | 'SUB' | 'WARNING',
        );
        const proposedSnapshot = rewriteSnapshotForTagMerge(
          (audit.proposed_snapshot as SiteAuditSnapshot | null) ?? null,
          sourceTag.id,
          targetTag.id,
          sourceTag.tag_type as 'MAIN' | 'SUB' | 'WARNING',
        );
        const nextDiff: SiteAuditDiffItem[] = proposedSnapshot
          ? buildSnapshotDiff(currentSnapshot, proposedSnapshot)
          : ((audit.diff as SiteAuditDiffItem[] | null) ?? []).map((item) => ({
              ...item,
              before: rewriteDiffValueForTagMerge(item.before, sourceTag.id, targetTag.id),
              after: rewriteDiffValueForTagMerge(item.after, sourceTag.id, targetTag.id),
            }));

        const changed =
          JSON.stringify(currentSnapshot) !== JSON.stringify(audit.current_snapshot) ||
          JSON.stringify(proposedSnapshot) !== JSON.stringify(audit.proposed_snapshot) ||
          JSON.stringify(nextDiff) !== JSON.stringify(audit.diff);

        if (!changed) {
          continue;
        }

        await app.db.write
          .update(SiteAudits)
          .set({
            current_snapshot: currentSnapshot,
            proposed_snapshot: proposedSnapshot,
            diff: nextDiff,
            updated_time: new Date(),
          })
          .where(eq(SiteAudits.id, audit.id));

        rewrittenAudits += 1;
      }

      await app.db.write
        .update(TagDefinitions)
        .set({
          is_enabled: false,
          merged_into_tag_id: targetTag.id,
          merged_by: actor.id,
          merged_time: new Date(),
          updated_time: new Date(),
        })
        .where(eq(TagDefinitions.id, sourceTag.id));
      await invalidatePublicSiteCache(app);

      return {
        ok: true,
        data: {
          source_tag_id: sourceTag.id,
          target_tag_id: targetTag.id,
          migrated_site_refs: migratedSiteRefs,
          migrated_warning_refs: migratedWarningRefs,
          rewritten_pending_audits: rewrittenAudits,
        },
      };
    },
  );
}

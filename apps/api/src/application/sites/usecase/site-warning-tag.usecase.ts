import {
  type SiteWarningTagMachineKey,
  SiteWarningTags,
  type SiteWarningTagSource,
  TagDefinitions,
} from '@zhblogs/db';

import { and, asc, eq, inArray } from 'drizzle-orm';
import type { FastifyInstance } from 'fastify';

import { invalidatePublicSiteCache } from '@/application/public/usecase/public-cache.usecase';

const SITE_WARNING_TAG_QUERY_BATCH_SIZE = 500;

export type SiteWarningTagDefinition = {
  id: string;
  machineKey: SiteWarningTagMachineKey;
  name: string;
  description: string | null;
};

export type SiteWarningTagEntry = SiteWarningTagDefinition & {
  siteId: string;
  source: SiteWarningTagSource;
  note: string | null;
};

type UpsertSiteWarningTagInput = {
  siteId: string;
  machineKey: SiteWarningTagMachineKey;
  source: SiteWarningTagSource;
  sourceSiteAuditId?: string | null;
  sourceArticleFeedbackAuditId?: string | null;
  note?: string | null;
  createdBy?: string | null;
};

export async function getWarningTagByMachineKey(
  app: FastifyInstance,
  machineKey: SiteWarningTagMachineKey,
): Promise<SiteWarningTagDefinition | null> {
  const [tag] = await app.db.read
    .select({
      id: TagDefinitions.id,
      machineKey: TagDefinitions.machine_key,
      name: TagDefinitions.name,
      description: TagDefinitions.description,
    })
    .from(TagDefinitions)
    .where(
      and(
        eq(TagDefinitions.tag_type, 'WARNING'),
        eq(TagDefinitions.is_enabled, true),
        eq(TagDefinitions.machine_key, machineKey),
      ),
    )
    .limit(1);

  if (!tag?.machineKey) {
    return null;
  }

  return {
    id: tag.id,
    machineKey: tag.machineKey as SiteWarningTagMachineKey,
    name: tag.name,
    description: tag.description ?? null,
  };
}

export async function upsertSiteWarningTag(
  app: FastifyInstance,
  input: UpsertSiteWarningTagInput,
): Promise<SiteWarningTagEntry> {
  const definition = await getWarningTagByMachineKey(app, input.machineKey);

  if (!definition) {
    throw new Error(`warning tag is not configured: ${input.machineKey}`);
  }

  const [row] = await app.db.write
    .insert(SiteWarningTags)
    .values({
      site_id: input.siteId,
      tag_id: definition.id,
      source: input.source,
      source_site_audit_id: input.sourceSiteAuditId ?? null,
      source_article_feedback_audit_id: input.sourceArticleFeedbackAuditId ?? null,
      note: input.note?.trim() || null,
      created_by: input.createdBy ?? null,
    })
    .onConflictDoUpdate({
      target: [SiteWarningTags.site_id, SiteWarningTags.tag_id, SiteWarningTags.source],
      set: {
        source_site_audit_id: input.sourceSiteAuditId ?? null,
        source_article_feedback_audit_id: input.sourceArticleFeedbackAuditId ?? null,
        note: input.note?.trim() || null,
        created_by: input.createdBy ?? null,
        updated_time: new Date(),
      },
    })
    .returning({
      siteId: SiteWarningTags.site_id,
      source: SiteWarningTags.source,
      note: SiteWarningTags.note,
    });

  if (!row) {
    throw new Error('failed to upsert site warning tag');
  }

  await invalidatePublicSiteCache(app);

  return {
    ...definition,
    siteId: row.siteId,
    source: row.source as SiteWarningTagSource,
    note: row.note ?? null,
  };
}

export async function listSiteWarningTags(
  app: FastifyInstance,
  siteId: string,
): Promise<SiteWarningTagEntry[]> {
  return listSiteWarningTagsBySiteIds(app, [siteId]).then((rows) =>
    rows.filter((row) => row.siteId === siteId),
  );
}

export async function listSiteWarningTagsBySiteIds(
  app: FastifyInstance,
  siteIds: string[],
): Promise<SiteWarningTagEntry[]> {
  const normalizedSiteIds = [...new Set(siteIds.filter((siteId) => siteId.length > 0))];

  if (normalizedSiteIds.length === 0) {
    return [];
  }

  const rows = (
    await Promise.all(
      chunkSiteIds(normalizedSiteIds, SITE_WARNING_TAG_QUERY_BATCH_SIZE).map((siteIdBatch) =>
        app.db.read
          .select({
            siteId: SiteWarningTags.site_id,
            source: SiteWarningTags.source,
            note: SiteWarningTags.note,
            createdTime: SiteWarningTags.created_time,
            id: TagDefinitions.id,
            machineKey: TagDefinitions.machine_key,
            name: TagDefinitions.name,
            description: TagDefinitions.description,
          })
          .from(SiteWarningTags)
          .innerJoin(TagDefinitions, eq(SiteWarningTags.tag_id, TagDefinitions.id))
          .where(
            and(
              inArray(SiteWarningTags.site_id, siteIdBatch),
              eq(TagDefinitions.tag_type, 'WARNING'),
              eq(TagDefinitions.is_enabled, true),
            ),
          )
          .orderBy(asc(TagDefinitions.name), asc(SiteWarningTags.created_time)),
      ),
    )
  )
    .flat()
    .sort((left, right) => {
      const nameCompare = left.name.localeCompare(right.name, 'zh-CN');

      if (nameCompare !== 0) {
        return nameCompare;
      }

      return left.createdTime.getTime() - right.createdTime.getTime();
    });

  return rows
    .filter((row) => row.machineKey)
    .map((row) => ({
      siteId: row.siteId,
      source: row.source as SiteWarningTagSource,
      note: row.note ?? null,
      id: row.id,
      machineKey: row.machineKey as SiteWarningTagMachineKey,
      name: row.name,
      description: row.description ?? null,
    }));
}

function chunkSiteIds(siteIds: string[], batchSize: number): string[][] {
  const chunks: string[][] = [];

  for (let index = 0; index < siteIds.length; index += batchSize) {
    chunks.push(siteIds.slice(index, index + batchSize));
  }

  return chunks;
}

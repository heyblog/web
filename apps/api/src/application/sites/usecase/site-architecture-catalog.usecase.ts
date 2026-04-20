import { Programs, TagDefinitions, TechnologyCatalogs } from '@zhblogs/db';

import { and, eq, inArray } from 'drizzle-orm';
import type { FastifyInstance } from 'fastify';

import { invalidatePublicSiteCache } from '@/application/public/usecase/public-cache.usecase';
import { normalizeSubTagSnapshots } from '@/domain/sites/service/site-snapshot-diff.service';

const normalizeArchitectureName = (value: string | null | undefined) => {
  const normalized = value?.trim() ?? '';
  return normalized.length > 0 ? normalized : null;
};

const normalizeArchitectureToken = (value: string | null | undefined) => {
  const normalized = normalizeArchitectureName(value);
  if (!normalized) {
    return null;
  }

  const compact = normalized.toLocaleLowerCase('zh-CN').replace(/[^\p{L}\p{N}]+/gu, '');

  return compact || normalized.toLocaleLowerCase('zh-CN');
};

export { normalizeArchitectureName, normalizeArchitectureToken };

export async function ensureSubTagIdsBySnapshot(
  app: FastifyInstance,
  subTags:
    | Array<{
        tag_id?: string | null;
        name?: string | null;
        name_normalized?: string | null;
      }>
    | null
    | undefined,
): Promise<string[]> {
  const normalized = normalizeSubTagSnapshots(subTags) ?? [];

  if (normalized.length === 0) {
    return [];
  }

  const selectedIds = normalized.map((item) => item.tag_id).filter(Boolean) as string[];
  const customNames = normalized
    .filter((item) => !item.tag_id && item.name)
    .map((item) => item.name as string);

  if (customNames.length === 0) {
    return [...new Set(selectedIds)].sort();
  }

  const existing = await app.db.read
    .select({
      id: TagDefinitions.id,
      name: TagDefinitions.name,
    })
    .from(TagDefinitions)
    .where(and(inArray(TagDefinitions.name, customNames), eq(TagDefinitions.tag_type, 'SUB')));

  const idByName = new Map(existing.map((row) => [row.name, row.id]));
  const missing = customNames.filter((name) => !idByName.has(name));

  if (missing.length > 0) {
    const inserted = await app.db.write
      .insert(TagDefinitions)
      .values(
        missing.map((name) => ({
          name,
          tag_type: 'SUB',
          is_enabled: true,
        })),
      )
      .returning({
        id: TagDefinitions.id,
        name: TagDefinitions.name,
      });

    for (const row of inserted) {
      idByName.set(row.name, row.id);
    }

    await invalidatePublicSiteCache(app);
  }

  return [
    ...new Set([...selectedIds, ...customNames.map((name) => idByName.get(name)).filter(Boolean)]),
  ].sort() as string[];
}

export async function ensureTechnologyCatalogIdByInput(
  app: FastifyInstance,
  technologyType: 'FRAMEWORK' | 'LANGUAGE',
  existingId: string | null | undefined,
  customName: string | null | undefined,
): Promise<string | null> {
  if (existingId) {
    return existingId;
  }

  const displayName = normalizeArchitectureName(customName);
  const normalizedName = normalizeArchitectureToken(customName);

  if (!displayName || !normalizedName) {
    return null;
  }

  const [existing] = await app.db.read
    .select({
      id: TechnologyCatalogs.id,
    })
    .from(TechnologyCatalogs)
    .where(
      and(
        eq(TechnologyCatalogs.name_normalized, normalizedName),
        eq(TechnologyCatalogs.technology_type, technologyType),
      ),
    )
    .limit(1);

  if (existing) {
    return existing.id;
  }

  const [inserted] = await app.db.write
    .insert(TechnologyCatalogs)
    .values({
      name: displayName,
      name_normalized: normalizedName,
      technology_type: technologyType,
      is_enabled: true,
    })
    .returning({
      id: TechnologyCatalogs.id,
    });

  return inserted?.id ?? null;
}

export async function ensureProgramByInput(
  app: FastifyInstance,
  existingId: string | null | undefined,
  customName: string | null | undefined,
  programIsOpenSource: boolean | null | undefined,
  websiteUrl: string | null | undefined,
  repoUrl: string | null | undefined,
): Promise<{ program_id: string | null; created: boolean }> {
  const normalizedWebsite = websiteUrl?.trim() || null;
  const normalizedRepo = repoUrl?.trim() || null;
  const resolvedWebsite = normalizedWebsite || normalizedRepo;

  if (existingId) {
    const [currentProgram] = await app.db.read
      .select({
        website_url: Programs.website_url,
        repo_url: Programs.repo_url,
        is_open_source: Programs.is_open_source,
      })
      .from(Programs)
      .where(eq(Programs.id, existingId))
      .limit(1);

    if (currentProgram) {
      const nextWebsite = currentProgram.website_url || resolvedWebsite;
      const nextRepo = currentProgram.repo_url || normalizedRepo;
      const nextIsOpenSource =
        programIsOpenSource === undefined
          ? currentProgram.is_open_source
          : (programIsOpenSource ?? currentProgram.is_open_source);

      if (
        nextWebsite !== currentProgram.website_url ||
        nextRepo !== currentProgram.repo_url ||
        nextIsOpenSource !== currentProgram.is_open_source
      ) {
        await app.db.write
          .update(Programs)
          .set({
            website_url: nextWebsite,
            repo_url: nextRepo,
            is_open_source: nextIsOpenSource,
            updated_time: new Date(),
          })
          .where(eq(Programs.id, existingId));
        await invalidatePublicSiteCache(app);
      }
    }

    if (!currentProgram && (resolvedWebsite || normalizedRepo || programIsOpenSource != null)) {
      await app.db.write
        .update(Programs)
        .set({
          website_url: resolvedWebsite,
          repo_url: normalizedRepo,
          is_open_source: programIsOpenSource ?? undefined,
          updated_time: new Date(),
        })
        .where(eq(Programs.id, existingId));
      await invalidatePublicSiteCache(app);
    }

    return {
      program_id: existingId,
      created: false,
    };
  }

  const displayName = normalizeArchitectureName(customName);
  const normalizedName = normalizeArchitectureToken(customName);

  if (!displayName || !normalizedName) {
    return {
      program_id: null,
      created: false,
    };
  }

  const [existing] = await app.db.read
    .select({
      id: Programs.id,
      website_url: Programs.website_url,
      repo_url: Programs.repo_url,
      is_open_source: Programs.is_open_source,
    })
    .from(Programs)
    .where(eq(Programs.name_normalized, normalizedName))
    .limit(1);

  if (existing) {
    const nextWebsite = existing.website_url || resolvedWebsite;
    const nextRepo = existing.repo_url || normalizedRepo;
    const nextIsOpenSource = existing.is_open_source ?? programIsOpenSource ?? undefined;

    if (
      nextWebsite !== existing.website_url ||
      nextRepo !== existing.repo_url ||
      nextIsOpenSource !== existing.is_open_source
    ) {
      await app.db.write
        .update(Programs)
        .set({
          website_url: nextWebsite,
          repo_url: nextRepo,
          is_open_source: nextIsOpenSource,
          updated_time: new Date(),
        })
        .where(eq(Programs.id, existing.id));
      await invalidatePublicSiteCache(app);
    }

    return {
      program_id: existing.id,
      created: false,
    };
  }

  const [inserted] = await app.db.write
    .insert(Programs)
    .values({
      name: displayName,
      name_normalized: normalizedName,
      is_open_source: programIsOpenSource ?? undefined,
      website_url: resolvedWebsite,
      repo_url: normalizedRepo,
      is_enabled: true,
    })
    .returning({ id: Programs.id });

  if (inserted?.id) {
    await invalidatePublicSiteCache(app);
  }

  return {
    program_id: inserted?.id ?? null,
    created: Boolean(inserted?.id),
  };
}

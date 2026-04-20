#!/usr/bin/env tsx

import { dbWrite, type ScheduleModeKey, TaskSchedules, type TaskTypeKey } from '@zhblogs/db';

import { eq, inArray } from 'drizzle-orm';

type ScriptMode = 'fill' | 'sync';

type ScriptOptions = {
  mode: ScriptMode;
  dryRun: boolean;
};

const HELP_TEXT = `
Seed task schedules from task catalog presets.

Usage:
  pnpm -F @zhblogs/api run tasks:seed
  pnpm -F @zhblogs/api run tasks:seed -- --sync
  pnpm -F @zhblogs/api run tasks:seed -- --dry-run

Options:
  --sync     Sync existing schedule definitions with preset values.
  --dry-run  Print planned writes without changing database.
  --help     Show this message.
`.trim();

const OBSOLETE_SCHEDULE_NAMES = [
  '站点检测分发（每 30 分钟）',
  'RSS 抓取分发（每 1 小时）',
  '上游同步（每 6 小时）',
  '统计汇总（每日离峰）',
  '任务健康巡检（每 2 小时）',
] as const;

const parseArgs = (argv: string[]): ScriptOptions => {
  const options: ScriptOptions = {
    mode: 'fill',
    dryRun: false,
  };

  for (const rawArg of argv) {
    const arg = rawArg.trim();

    if (!arg || arg === '--') {
      continue;
    }

    if (arg === '--help' || arg === '-h') {
      console.info(HELP_TEXT);
      process.exit(0);
    }

    if (arg === '--sync') {
      options.mode = 'sync';
      continue;
    }

    if (arg === '--dry-run') {
      options.dryRun = true;
      continue;
    }

    throw new Error(`Unsupported argument: ${arg}`);
  }

  return options;
};

const assertDatabaseUrl = (): void => {
  const value = process.env.DATABASE_URL?.trim();
  if (!value) {
    throw new Error(
      'Missing DATABASE_URL. Run from the repo root with `pnpm run tasks:seed` or use `node --env-file=.env.dev --import tsx ...`.',
    );
  }
};

const normalizeJson = (value: unknown): unknown => {
  if (Array.isArray(value)) {
    return value.map((item) => normalizeJson(item));
  }

  if (value && typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([key, item]) => [key, normalizeJson(item)]);
    return Object.fromEntries(entries);
  }

  return value;
};

const isJsonEqual = (left: unknown, right: unknown): boolean =>
  JSON.stringify(normalizeJson(left)) === JSON.stringify(normalizeJson(right));

const getDbClient = (): { end?: () => Promise<void> } | null => {
  if (typeof dbWrite !== 'object' || dbWrite === null) {
    return null;
  }
  return (dbWrite as { $client?: { end?: () => Promise<void> } }).$client ?? null;
};

const main = async () => {
  const options = parseArgs(process.argv.slice(2));
  assertDatabaseUrl();

  const presets: Array<{
    name: string;
    task_type: TaskTypeKey;
    schedule_mode: ScheduleModeKey;
    schedule_config: Record<string, unknown>;
    payload_template: Record<string, unknown>;
  }> = [];
  const presetNames = presets.map((preset) => preset.name);
  const obsoleteNames = OBSOLETE_SCHEDULE_NAMES.filter((name) => !presetNames.includes(name));

  const existingRows = await dbWrite
    .select()
    .from(TaskSchedules)
    .where(inArray(TaskSchedules.name, presetNames.length > 0 ? presetNames : ['__never_match__']));

  const obsoleteRows =
    obsoleteNames.length === 0
      ? []
      : await dbWrite
          .select({
            id: TaskSchedules.id,
            name: TaskSchedules.name,
          })
          .from(TaskSchedules)
          .where(inArray(TaskSchedules.name, obsoleteNames));

  const existingByName = new Map(existingRows.map((row) => [row.name, row]));

  let created = 0;
  let updated = 0;
  let skipped = 0;
  let unchanged = 0;
  let fillOutdated = 0;
  let deletedObsolete = 0;

  for (const preset of presets) {
    const existing = existingByName.get(preset.name);

    if (!existing) {
      created += 1;
      if (options.dryRun) {
        console.info(`[dry-run] create schedule: ${preset.name}`);
        continue;
      }

      await dbWrite.insert(TaskSchedules).values({
        name: preset.name,
        task_type: preset.task_type,
        schedule_mode: preset.schedule_mode,
        is_enabled: true,
        schedule_config: preset.schedule_config,
        payload_template: preset.payload_template,
        next_run_time: new Date(),
      });
      console.info(`created schedule: ${preset.name}`);
      continue;
    }

    const changed =
      existing.task_type !== preset.task_type ||
      existing.schedule_mode !== preset.schedule_mode ||
      !isJsonEqual(existing.schedule_config, preset.schedule_config) ||
      !isJsonEqual(existing.payload_template, preset.payload_template);

    if (options.mode === 'fill') {
      if (changed) {
        fillOutdated += 1;
        console.warn(
          `[fill-mode] schedule preset differs in DB: ${preset.name}. Run tasks:seed:sync to apply updates.`,
        );
      }
      skipped += 1;
      continue;
    }

    if (!changed) {
      unchanged += 1;
      continue;
    }

    updated += 1;
    if (options.dryRun) {
      console.info(`[dry-run] update schedule: ${preset.name}`);
      continue;
    }

    await dbWrite
      .update(TaskSchedules)
      .set({
        task_type: preset.task_type,
        schedule_mode: preset.schedule_mode,
        schedule_config: preset.schedule_config,
        payload_template: preset.payload_template,
      })
      .where(eq(TaskSchedules.id, existing.id));
    console.info(`updated schedule: ${preset.name}`);
  }

  if (presetNames.length === 0 && options.mode !== 'sync') {
    console.info('No schedule presets found.');
  }

  if (options.mode === 'sync') {
    for (const obsolete of obsoleteRows) {
      deletedObsolete += 1;
      if (options.dryRun) {
        console.info(`[dry-run] delete obsolete schedule: ${obsolete.name}`);
        continue;
      }

      await dbWrite.delete(TaskSchedules).where(eq(TaskSchedules.id, obsolete.id));
      console.info(`deleted obsolete schedule: ${obsolete.name}`);
    }
  } else if (obsoleteRows.length > 0) {
    console.warn(
      `[fill-mode] found ${obsoleteRows.length} obsolete schedules. Run tasks:seed:sync to remove them.`,
    );
  }

  console.info(
    [
      `mode=${options.mode}`,
      `dryRun=${String(options.dryRun)}`,
      `created=${created}`,
      `updated=${updated}`,
      `skipped=${skipped}`,
      `unchanged=${unchanged}`,
      `fill_outdated=${fillOutdated}`,
      `deleted_obsolete=${deletedObsolete}`,
    ].join(' '),
  );
};

main()
  .catch((error: unknown) => {
    console.error(error);
    process.exitCode = 1;
  })
  .finally(async () => {
    const client = getDbClient();
    if (client?.end) {
      await client.end();
    }
  });

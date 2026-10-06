import {
  isRecord,
  isRevision,
  isSlugConflict,
  isUUID,
  type SlugConflict,
} from './taxonomy.types.ts';

export type SlugJobStatus = 'queued' | 'running' | 'paused' | 'ready' | 'cancelled' | 'completed';
export type SlugItemState =
  'pending' | 'running' | 'ready' | 'failed' | 'stale' | 'applied' | 'needs_confirmation';
export interface SlugJobItem {
  readonly conflicts: readonly SlugConflict[];
  readonly tag_id: string;
  readonly name: string;
  readonly original_slug: string;
  readonly slug: string;
  readonly state: SlugItemState;
  readonly error_code: string;
  readonly source: string;
}
export interface SlugJob {
  readonly id: string;
  readonly status: SlugJobStatus;
  readonly revision: string;
  readonly model_id: string;
  readonly pause_code: string;
  readonly resume_after: string;
  readonly counts: {
    readonly total: number;
    readonly ready: number;
    readonly needs_confirmation: number;
    readonly failed: number;
    readonly applied: number;
  };
  readonly items: readonly SlugJobItem[];
}
export type SlugSelection =
  | { readonly kind: 'invalid' | 'all' }
  | { readonly kind: 'ids'; readonly ids: readonly string[] }
  | {
      readonly kind: 'filter';
      readonly filter: { readonly query?: string; readonly is_enabled?: boolean };
    };
const statuses: readonly unknown[] = [
  'queued',
  'running',
  'paused',
  'ready',
  'cancelled',
  'completed',
];
const states: readonly unknown[] = [
  'pending',
  'running',
  'ready',
  'failed',
  'stale',
  'applied',
  'needs_confirmation',
];
function isItem(value: unknown): value is SlugJobItem {
  return (
    isRecord(value) &&
    isUUID(value.tag_id) &&
    typeof value.name === 'string' &&
    typeof value.original_slug === 'string' &&
    typeof value.slug === 'string' &&
    states.includes(value.state) &&
    typeof value.error_code === 'string' &&
    typeof value.source === 'string' &&
    Array.isArray(value.conflicts) &&
    value.conflicts.every(isSlugConflict)
  );
}
function isJob(value: unknown): value is SlugJob {
  return (
    isRecord(value) &&
    isUUID(value.id) &&
    statuses.includes(value.status) &&
    isRevision(value.revision) &&
    typeof value.model_id === 'string' &&
    typeof value.pause_code === 'string' &&
    typeof value.resume_after === 'string' &&
    isRecord(value.counts) &&
    ['total', 'ready', 'failed', 'applied', 'needs_confirmation'].every((key) => {
      const count = value.counts;
      return (
        isRecord(count) &&
        typeof count[key] === 'number' &&
        Number.isSafeInteger(count[key]) &&
        count[key] >= 0
      );
    }) &&
    Array.isArray(value.items) &&
    value.items.every(isItem)
  );
}
export const parseSlugJob = (value: unknown): SlugJob | null => (isJob(value) ? value : null);
export function parseSlugJobs(value: unknown): readonly SlugJob[] | null {
  return isRecord(value) && Array.isArray(value.jobs) && value.jobs.every(isJob)
    ? value.jobs
    : null;
}

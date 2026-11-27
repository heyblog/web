import { parseConfiguration } from './configuration.types.ts';
import { bool, choice, list, nullable, number, record, strings, text } from './parse.ts';

export {
  type Configuration,
  parseConfiguration,
  parseSchedule,
  type Schedule,
  type ScheduleInput,
} from './configuration.types.ts';
export const runStates = [
  'queued',
  'running',
  'pausing',
  'paused',
  'cancelling',
  'succeeded',
  'partial_failed',
  'failed',
  'timed_out',
  'cancelled',
] as const;
export const controls = ['pause', 'resume', 'cancel', 'archive', 'unarchive'] as const;
export type Control = (typeof controls)[number];
export type RetrySelection = 'failed' | 'unfinished' | 'all';
export interface TaskQuery {
  readonly page?: number;
  readonly page_size?: number;
  readonly kind?: string;
  readonly state?: string;
  readonly source?: string;
  readonly schedule?: string;
  readonly from?: string;
  readonly to?: string;
  readonly archived?: string;
}
export function taskQuery(query: TaskQuery = {}): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== '') params.set(key, String(value));
  }
  return params.size ? `?${params}` : '';
}
export function parsePage<T>(value: unknown, parse: (item: unknown) => T) {
  const v = record(value);
  return {
    items: list(v.items, parse),
    total: number(v.total),
    page: number(v.page),
    page_size: number(v.page_size),
  };
}
export type Page<T> = ReturnType<typeof parsePage<T>>;
export function parseCounts(value: unknown) {
  const v = record(value);
  return {
    total: number(v.total),
    succeeded: number(v.succeeded),
    failed: number(v.failed),
    skipped: number(v.skipped),
    cancelled: number(v.cancelled),
    pending: number(v.pending),
    usable: number(v.usable),
  };
}
export function parseRun(value: unknown) {
  const v = record(value);
  const cleaned = nullable(v.cleaned_at);
  return {
    id: text(v.id),
    schedule_id: nullable(v.schedule_id),
    kind: choice(v.kind, ['monitor', 'rss', 'verification']),
    state: choice(v.state, runStates),
    snapshot: cleaned ? null : parseConfiguration(v.snapshot),
    scheduled_at: text(v.scheduled_at),
    created_at: text(v.created_at),
    started_at: nullable(v.started_at),
    finished_at: nullable(v.finished_at),
    paused_at: nullable(v.paused_at),
    paused_ms: number(v.paused_ms),
    timeout_seconds: number(v.timeout_seconds),
    elapsed_ms: number(v.elapsed_ms),
    queue_ms: number(v.queue_ms),
    source: choice(v.source, ['scheduled', 'manual', 'event', 'retry', 'rerun']),
    actor_id: nullable(v.actor_id),
    root_run_id: nullable(v.root_run_id),
    parent_run_id: nullable(v.parent_run_id),
    retry_number: number(v.retry_number),
    archived: bool(v.archived),
    cleaned_at: cleaned,
    reason: text(v.reason),
    counts: parseCounts(v.counts),
    allowed_actions: list(v.allowed_actions, text),
  };
}
export type Run = ReturnType<typeof parseRun>;
export function parsePath(value: unknown) {
  const v = record(value);
  return {
    platform: choice(v.platform, ['CF', 'EO']),
    outcome: choice(v.outcome, ['OK', 'FAIL', 'UNKNOWN']),
    http_status: number(v.http_status),
    error_kind: v.error_kind === undefined ? '' : text(v.error_kind),
    requests: number(v.requests),
    duration_ms: number(v.duration_ms),
    redirects: v.redirects === null ? [] : list(v.redirects, text),
    node: v.node === null ? {} : strings(v.node),
    observed_at: v.observed_at == null ? null : text(v.observed_at),
  };
}
export function parseResult(value: unknown) {
  if (value === null) return null;
  const v = record(value);
  return {
    paths: list(v.paths, parsePath),
    fallback: bool(v.fallback),
    business_valid: bool(v.business_valid),
  };
}
export function parseTarget(value: unknown) {
  const v = record(value);
  return {
    id: text(v.id),
    site_id: text(v.site_id),
    address: text(v.address),
    kind: text(v.kind),
    state: text(v.state),
    sample_started_at: nullable(v.sample_started_at),
    reason: text(v.reason),
    usable: bool(v.usable),
    recoveries: number(v.recoveries),
    round_id: text(v.round_id),
    result: parseResult(v.result),
  };
}
export type Target = ReturnType<typeof parseTarget>;
export function parseAccepted(value: unknown) {
  return text(record(value).id);
}
export function parseBatchItem(value: unknown) {
  const v = record(value);
  return { id: text(v.id), state: text(v.state), code: text(v.code) };
}
export type BatchItem = ReturnType<typeof parseBatchItem>;

import { bool, choice, list, nullable, number, record, strings, text } from './parse.ts';

export function parseConfiguration(value: unknown) {
  const v = record(value);
  return {
    mode: choice(v.mode || 'interval', ['interval', 'cron']),
    interval_seconds: number(v.interval_seconds ?? 3600),
    cron: text(v.cron ?? ''),
    timezone: text(v.timezone || 'Asia/Shanghai'),
    scope: choice(v.scope || 'all', ['all', 'selected']),
    site_ids: v.site_ids == null ? [] : list(v.site_ids, text),
    request_config_id: text(v.request_config_id ?? ''),
    feed_mode: choice(v.feed_mode || 'default', ['default', 'all']),
    user_agent: text(v.user_agent),
    headers: v.headers === null ? {} : strings(v.headers),
    timeout_seconds: number(v.timeout_seconds),
    interval_ms: number(v.interval_ms),
    concurrency: number(v.concurrency ?? 1),
    retries: number(v.retries),
    max_redirects: number(v.max_redirects ?? 3),
    run_timeout_seconds: number(v.run_timeout_seconds ?? 86400),
    signed: bool(v.signed),
    carrier: choice(v.carrier || 'both', ['both', 'header']),
    ttl_seconds: number(v.ttl_seconds),
  };
}
export type Configuration = ReturnType<typeof parseConfiguration>;
export interface ScheduleInput {
  readonly name: string;
  readonly kind: 'monitor' | 'rss';
  readonly enabled: boolean;
  readonly archived: boolean;
  readonly revision: number;
  readonly configuration: Configuration;
}
export function parseSchedule(value: unknown) {
  const v = record(value);
  return {
    id: text(v.id),
    name: text(v.name),
    kind: choice(v.kind, ['monitor', 'rss']),
    enabled: bool(v.enabled),
    archived: bool(v.archived),
    revision: number(v.revision),
    configuration: parseConfiguration(v.configuration),
    next_at: nullable(v.next_at),
  };
}
export type Schedule = ReturnType<typeof parseSchedule>;
export interface RequestConfigInput {
  readonly name: string;
  readonly purpose: 'monitor' | 'rss' | 'verification';
  readonly enabled: boolean;
  readonly archived: boolean;
  readonly revision: number;
  readonly parameters: Configuration;
}
export function parseRequestConfig(value: unknown) {
  const v = record(value);
  return {
    id: text(v.id),
    name: text(v.name),
    purpose: choice(v.purpose, ['monitor', 'rss', 'verification']),
    enabled: bool(v.enabled),
    archived: bool(v.archived),
    revision: number(v.revision),
    parameters: parseConfiguration(v.parameters),
    references: list(v.references, text),
    verification_default: bool(v.verification_default),
  };
}
export type RequestConfig = ReturnType<typeof parseRequestConfig>;
export interface ManualInput {
  readonly kind: 'monitor' | 'rss';
  readonly configuration: Configuration;
}

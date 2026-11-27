import { bool, list, nullable, number, record, text } from './parse.ts';
import { parseRun } from './tasks.types.ts';

export function parseWorker(value: unknown) {
  const v = record(value);
  return {
    instance_id: text(v.instance_id),
    protocol: number(v.protocol),
    version: text(v.version),
    registered_at: text(v.registered_at),
    heartbeat_at: text(v.heartbeat_at),
    draining: bool(v.draining),
    online: bool(v.online),
    active: number(v.active),
  };
}
export type Worker = ReturnType<typeof parseWorker>;
export function parseOverview(value: unknown) {
  const v = record(value);
  return {
    states: Object.fromEntries(
      Object.entries(record(v.states)).map(([key, value]) => [key, number(value)]),
    ),
    recent: list(v.recent, parseRun),
    workers: list(v.workers, parseWorker),
    backoff: list(v.backoff, (value) => {
      const b = record(value);
      return {
        platform: text(b.platform),
        retry_at: nullable(b.retry_at),
        reason: text(b.reason),
        stage: number(b.stage),
      };
    }),
  };
}
export type Overview = ReturnType<typeof parseOverview>;
// Status counts and total duration describe request hops. Retries deduplicate
// (target, platform, logical_retry > 0); redirects count hops > 0; fallbacks count
// distinct targets on the alternate platform. These are observations, not billing.
export function parseStatistics(value: unknown) {
  const v = record(value);
  return {
    platform: text(v.platform),
    kind: text(v.kind),
    authorized: number(v.authorized),
    sent: number(v.sent),
    received: number(v.received),
    succeeded: number(v.succeeded),
    failed: number(v.failed),
    unknown: number(v.unknown),
    duration_ms: number(v.duration_ms),
    retries: number(v.retries),
    redirects: number(v.redirects),
    fallbacks: number(v.fallbacks),
  };
}
export type Statistics = ReturnType<typeof parseStatistics>;
export function parseAttempt(value: unknown) {
  const v = record(value);
  return {
    id: text(v.id),
    target_id: nullable(v.target_id),
    platform: text(v.platform),
    signed: bool(v.signed),
    state: text(v.state),
    url: text(v.url),
    method: text(v.method),
    retry: number(v.retry),
    hop: number(v.hop),
    parent_id: nullable(v.parent_id),
    key_id: nullable(v.key_id),
    policy_revision: number(v.policy_revision),
    created_at: text(v.created_at),
    sent_at: nullable(v.sent_at),
    received_at: nullable(v.received_at),
    accepted: bool(v.accepted),
  };
}
export type Attempt = ReturnType<typeof parseAttempt>;
export function parseTaskEvent(value: unknown) {
  const v = record(value);
  return {
    id: text(v.id),
    actor_id: nullable(v.actor_id),
    entity_id: nullable(v.entity_id),
    action: text(v.action),
    outcome: text(v.outcome),
    created_at: text(v.created_at),
  };
}
export type TaskEvent = ReturnType<typeof parseTaskEvent>;

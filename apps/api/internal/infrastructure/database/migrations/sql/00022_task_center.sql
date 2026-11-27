-- +goose Up
CREATE SCHEMA operations;
GRANT USAGE ON SCHEMA operations TO api_runtime;

CREATE TABLE operations.request_configs (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    purpose text NOT NULL CHECK (purpose IN ('monitor','rss','verification')),
    enabled boolean NOT NULL DEFAULT false,
    archived boolean NOT NULL DEFAULT false,
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    parameters jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE operations.task_settings (
    id integer PRIMARY KEY CHECK (id=1),
    verification_config_id uuid REFERENCES operations.request_configs(id)
);

CREATE TABLE operations.workers (
    id uuid PRIMARY KEY,
    protocol integer NOT NULL CHECK (protocol=2),
    version text NOT NULL CHECK (length(version) BETWEEN 1 AND 120),
    registered_at timestamptz NOT NULL DEFAULT now(),
    heartbeat_at timestamptz NOT NULL DEFAULT now(),
    draining boolean NOT NULL DEFAULT false
);

CREATE TABLE operations.schedules (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- Stable schedule identifier.
    name text NOT NULL, -- Administrator-facing schedule name.
    kind text NOT NULL CHECK (kind IN ('monitor', 'rss')), -- Operation: dual-region monitor or RSS fetch.
    enabled boolean NOT NULL DEFAULT false, -- Whether the scheduler may create subsequent runs.
    configuration jsonb NOT NULL, -- Validated scheduling and ordinary outbound request settings.
    next_at timestamptz, -- Next due instant in UTC; null when disabled.
    revision integer NOT NULL DEFAULT 1, -- Optimistic revision for concurrent administrator edits.
    created_at timestamptz NOT NULL DEFAULT now(), -- Schedule creation time.
    request_config_id uuid NOT NULL REFERENCES operations.request_configs(id),
    archived boolean NOT NULL DEFAULT false
);

CREATE TABLE operations.runs (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- Stable run and monitoring-round identifier.
    schedule_id uuid REFERENCES operations.schedules(id), -- Owning schedule.
    kind text NOT NULL CHECK (kind IN ('monitor', 'rss', 'verification')), -- Operation captured when the run was created.
    state text NOT NULL DEFAULT 'queued' CHECK (state IN ('queued','running','pausing','paused','cancelling','succeeded','partial_failed','failed','timed_out','cancelled')), -- Execution lifecycle, independent of target health.
    snapshot jsonb NOT NULL, -- Immutable ordinary request configuration; signing policy remains live.
    scheduled_at timestamptz NOT NULL, -- Logical occurrence instant used for idempotent run creation.
    created_at timestamptz NOT NULL DEFAULT now(), -- Actual run creation and monitoring-round ordering time.
    finished_at timestamptz, -- Completion or cancellation time.
    source text NOT NULL DEFAULT 'scheduled' CHECK (source IN ('scheduled','manual','event','retry','rerun')),
    actor_id uuid REFERENCES identity.users(id),
    request_config_id uuid REFERENCES operations.request_configs(id),
    root_run_id uuid REFERENCES operations.runs(id) DEFERRABLE INITIALLY DEFERRED,
    parent_run_id uuid REFERENCES operations.runs(id) DEFERRABLE INITIALLY DEFERRED,
    retry_number integer NOT NULL DEFAULT 0 CHECK (retry_number>=0),
    started_at timestamptz,
    paused_at timestamptz,
    paused_ms bigint NOT NULL DEFAULT 0 CHECK (paused_ms>=0),
    timeout_seconds integer NOT NULL DEFAULT 86400 CHECK (timeout_seconds BETWEEN 60 AND 604800),
    next_dispatch_at timestamptz,
    archived boolean NOT NULL DEFAULT false,
    cleaned_at timestamptz,
    reason text NOT NULL DEFAULT ''
);

CREATE TABLE operations.targets (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- Stable target-work identifier.
    run_id uuid NOT NULL REFERENCES operations.runs(id), -- Owning run.
    site_id uuid NOT NULL REFERENCES directory.sites(id), -- Canonical site captured for this operation.
    feed_id uuid REFERENCES directory.site_feeds(id) ON DELETE SET NULL, -- RSS feed identifier; null for site monitoring or a deleted feed.
    kind text NOT NULL CHECK (kind IN ('monitor', 'rss', 'verification')), -- Operation used to exclude concurrent work of the same kind.
    address text NOT NULL, -- Request address captured when the run was created.
    generation bigint NOT NULL, -- Address generation used to reject stale observations.
    access_scope text, -- Observed access scope at run creation; routing reads current stable scope.
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','running','retry_wait','succeeded','partial_failed','failed','timed_out','cancelled','skipped')), -- Target execution lifecycle.
    lease_token uuid, -- Worker-only fencing token, invalidated on recovery or cancellation.
    lease_until timestamptz, -- Lease expiry extended by matching heartbeats.
    available_at timestamptz NOT NULL DEFAULT now(), -- Earliest dispatch time for request spacing.
    progress jsonb NOT NULL DEFAULT '{}', -- Persisted route and consumed attempt checkpoints for interruption recovery.
    result jsonb, -- Completed path results without response bodies or internal diagnostics.
    finished_at timestamptz, -- Target completion or cancellation time.
    worker_id uuid REFERENCES operations.workers(id),
    claim_id uuid REFERENCES directory.site_claims(id) ON DELETE SET NULL,
    challenge_digest bytea,
    claim_snapshot jsonb,
    round_id uuid NOT NULL DEFAULT uuidv7(),
    sample_started_at timestamptz,
    recoveries integer NOT NULL DEFAULT 0 CHECK (recoveries BETWEEN 0 AND 4),
    reason text NOT NULL DEFAULT '',
    usable boolean NOT NULL DEFAULT false
);

CREATE TABLE operations.signing_policy (
    id integer PRIMARY KEY CHECK (id=1), -- Singleton key fixed to one.
    enabled boolean NOT NULL DEFAULT true, -- Whether every outbound request must carry a fresh proof.
    carrier text NOT NULL DEFAULT 'both' CHECK (carrier IN ('both','header')), -- Proof in both UA and header, or in the header alone.
    ttl_seconds integer NOT NULL DEFAULT 300 CHECK (ttl_seconds BETWEEN 60 AND 300), -- Proof validity duration from 60 to 300 seconds.
    revision integer NOT NULL DEFAULT 1
);

CREATE TABLE operations.signing_keys (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- Public key identifier and authenticated encryption binding.
    platform text NOT NULL CHECK (platform IN ('CF','EO')), -- Cloudflare CF or EdgeOne EO signing identity.
    public_key jsonb NOT NULL, -- Public ES256 JWK retained for historical verification.
    encrypted_private_key bytea NOT NULL, -- Encrypted private key; master encryption key is never stored in the database.
    state text NOT NULL DEFAULT 'generated' CHECK (state IN ('generated','published','active','retired','revoked')), -- Generated, published, active, retired or revoked key lifecycle.
    created_at timestamptz NOT NULL DEFAULT now(), -- Key generation time.
    published_at timestamptz, -- First public key publication time.
    revoked_at timestamptz -- Revocation time; restored keys are always revoked.
);

CREATE TABLE operations.signing_events (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- Signing-operation audit identifier.
    key_id uuid REFERENCES operations.signing_keys(id), -- Affected key, or null for policy changes.
    actor_id uuid REFERENCES identity.users(id), -- Administrator who performed the operation.
    action text NOT NULL, -- Stable operation name without secret material.
    created_at timestamptz NOT NULL DEFAULT now() -- Audit event creation time.
);

CREATE TABLE operations.request_attempts (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- Dispatch reservation identifier.
    target_id uuid REFERENCES operations.targets(id), -- Associated task target.
    platform text NOT NULL CHECK (platform IN ('CF','EO')), -- Platform executing the request.
    signed boolean NOT NULL, -- Whether live signing policy issued a proof for this dispatch.
    created_at timestamptz NOT NULL DEFAULT now(), -- Dispatch reservation time.
    request_key text,
    worker_id uuid REFERENCES operations.workers(id),
    lease_token uuid,
    kind text NOT NULL CHECK (kind IN ('monitor','rss','verification')),
    url text NOT NULL DEFAULT '',
    method text NOT NULL DEFAULT 'GET' CHECK (method IN ('GET','HEAD')),
    logical_retry integer NOT NULL DEFAULT 0 CHECK (logical_retry BETWEEN 0 AND 2),
    hop integer NOT NULL DEFAULT 0 CHECK (hop BETWEEN 0 AND 3),
    parent_id uuid REFERENCES operations.request_attempts(id) DEFERRABLE INITIALLY DEFERRED,
    key_id uuid REFERENCES operations.signing_keys(id),
    policy_revision integer NOT NULL DEFAULT 1,
    state text NOT NULL DEFAULT 'unknown' CHECK (state IN ('authorized','sent','received','unknown')),
    expires_at timestamptz,
    sent_at timestamptz,
    received_at timestamptz,
    response_digest text,
    response_meta jsonb,
    accepted boolean NOT NULL DEFAULT false
);

CREATE TABLE operations.feed_documents (
    feed_id uuid PRIMARY KEY REFERENCES directory.site_feeds(id) ON DELETE CASCADE, -- Canonical feed whose document was validated.
    etag text NOT NULL DEFAULT '', -- Validated document ETag for conditional requests.
    last_modified text NOT NULL DEFAULT '', -- Validated document Last-Modified header.
    checked_at timestamptz NOT NULL DEFAULT now() -- Last successful document storage time.
);

CREATE TABLE operations.feed_items (
    feed_id uuid NOT NULL REFERENCES directory.site_feeds(id) ON DELETE CASCADE, -- Canonical source feed.
    item_key text NOT NULL, -- SHA-256 of the source item identifier or canonical item URL.
    title text NOT NULL, -- Source item title.
    url text NOT NULL, -- Validated absolute HTTP or HTTPS item URL.
    published text NOT NULL, -- Source publication value retained without guessing its timezone.
    PRIMARY KEY(feed_id,item_key)
);

CREATE TABLE operations.route_state (
    site_id uuid NOT NULL REFERENCES directory.sites(id) ON DELETE CASCADE, -- Site whose next preference is stored.
    operation text NOT NULL, -- Independent RSS or ownership-verification routing sequence.
    next_cf boolean NOT NULL DEFAULT true, -- Whether the next unspecified-scope request prefers Cloudflare.
    PRIMARY KEY(site_id,operation)
);

CREATE TABLE operations.task_events (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    run_id uuid REFERENCES operations.runs(id),
    actor_id uuid REFERENCES identity.users(id),
    entity_id uuid,
    action text NOT NULL,
    outcome text NOT NULL DEFAULT 'accepted',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE operations.task_idempotency (
    scope text NOT NULL,
    key text NOT NULL CHECK (length(key) BETWEEN 1 AND 120),
    digest text NOT NULL,
    result jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(scope,key)
);

CREATE TABLE operations.platform_backoff (
    platform text PRIMARY KEY CHECK (platform IN ('CF','EO')),
    retry_at timestamptz,
    reason text NOT NULL DEFAULT '',
    stage integer NOT NULL DEFAULT 0 CHECK (stage BETWEEN 0 AND 6),
    probe_id uuid,
    probe_until timestamptz
);

CREATE TABLE operations.request_statistics (
    attempt_id uuid PRIMARY KEY,
    platform text NOT NULL CHECK (platform IN ('CF','EO')),
    kind text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    authorized boolean NOT NULL DEFAULT false,
    sent boolean NOT NULL DEFAULT false,
    received boolean NOT NULL DEFAULT false,
    succeeded boolean NOT NULL DEFAULT false,
    failed boolean NOT NULL DEFAULT false,
    unknown boolean NOT NULL DEFAULT false,
    duration_ms bigint NOT NULL DEFAULT 0 CHECK (duration_ms>=0),
    retries integer NOT NULL DEFAULT 0 CHECK (retries BETWEEN 0 AND 2),
    redirects integer NOT NULL DEFAULT 0 CHECK (redirects BETWEEN 0 AND 3),
    fallback boolean NOT NULL DEFAULT false
);

CREATE UNIQUE INDEX targets_run_site ON operations.targets(run_id,site_id) WHERE kind='monitor';
CREATE UNIQUE INDEX targets_run_feed ON operations.targets(run_id,feed_id) WHERE feed_id IS NOT NULL;
CREATE UNIQUE INDEX targets_site_active ON operations.targets(site_id,kind) WHERE state = 'running';
CREATE INDEX targets_available ON operations.targets(available_at) WHERE state IN ('pending','running');
CREATE UNIQUE INDEX signing_keys_active ON operations.signing_keys(platform) WHERE state='active';
CREATE UNIQUE INDEX runs_scheduled_occurrence ON operations.runs(schedule_id,scheduled_at) WHERE source='scheduled';
CREATE UNIQUE INDEX runs_one_active ON operations.runs(schedule_id)
    WHERE state IN ('queued','running','pausing','paused','cancelling');
CREATE UNIQUE INDEX targets_claim_active ON operations.targets(claim_id,challenge_digest)
    WHERE kind='verification' AND state IN ('pending','running','retry_wait');
CREATE UNIQUE INDEX attempts_request_key ON operations.request_attempts(target_id,request_key);
CREATE UNIQUE INDEX attempts_logical_hop ON operations.request_attempts(target_id,platform,logical_retry,hop) WHERE request_key IS NOT NULL;
CREATE INDEX task_events_run ON operations.task_events(run_id,created_at,id);
CREATE INDEX request_statistics_period ON operations.request_statistics(created_at,platform,kind);

INSERT INTO operations.signing_policy(id) VALUES (1);
INSERT INTO operations.platform_backoff(platform) VALUES ('CF'),('EO');
INSERT INTO operations.request_configs(name,purpose,enabled,parameters)
SELECT name,kind,true,'{"user_agent":"HeyBlog/1.0","headers":{},"timeout_seconds":8,"interval_ms":1000,"retries":0,"max_redirects":3,"signed":true,"carrier":"both","ttl_seconds":300}'::jsonb
FROM (VALUES ('站点双路检测','monitor'),('RSS 抓取','rss')) defaults(name,kind);
INSERT INTO operations.request_configs(name,purpose,enabled,parameters)
VALUES ('HTTP 认证','verification',true,
    '{"user_agent":"HeyBlog-Verification","headers":{},"timeout_seconds":8,"interval_ms":1000,"retries":0,"max_redirects":3,"signed":true,"carrier":"both","ttl_seconds":300}');
INSERT INTO operations.task_settings
SELECT 1,id FROM operations.request_configs WHERE purpose='verification';
INSERT INTO operations.schedules(name,kind,request_config_id,configuration)
SELECT name,purpose,id,jsonb_build_object('request_config_id',id)||
    '{"mode":"interval","interval_seconds":3600,"cron":"","timezone":"Asia/Shanghai","scope":"all","site_ids":[],"concurrency":2,"feed_mode":"all","run_timeout_seconds":86400}'::jsonb
FROM operations.request_configs WHERE purpose IN ('monitor','rss');
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA operations TO api_runtime;

COMMENT ON TABLE operations.schedules IS 'Validated schedules; disabled by default and independent of existing runs.';
COMMENT ON COLUMN operations.schedules.id IS 'Stable schedule identifier.';
COMMENT ON COLUMN operations.schedules.name IS 'Administrator-facing schedule name.';
COMMENT ON COLUMN operations.schedules.kind IS 'Operation: dual-region monitor or RSS fetch.';
COMMENT ON COLUMN operations.schedules.enabled IS 'Whether the scheduler may create subsequent runs.';
COMMENT ON COLUMN operations.schedules.configuration IS 'Validated scheduling and ordinary outbound request settings.';
COMMENT ON COLUMN operations.schedules.next_at IS 'Next due instant in UTC; null when disabled.';
COMMENT ON COLUMN operations.schedules.revision IS 'Optimistic revision for concurrent administrator edits.';
COMMENT ON COLUMN operations.schedules.created_at IS 'Schedule creation time.';
COMMENT ON TABLE operations.runs IS 'Idempotent schedule occurrences with immutable request snapshots.';
COMMENT ON COLUMN operations.runs.id IS 'Stable run and monitoring-round identifier.';
COMMENT ON COLUMN operations.runs.schedule_id IS 'Owning schedule.';
COMMENT ON COLUMN operations.runs.kind IS 'Operation captured when the run was created.';
COMMENT ON COLUMN operations.runs.state IS 'Execution lifecycle, independent of target health.';
COMMENT ON COLUMN operations.runs.snapshot IS 'Immutable ordinary request configuration; signing policy remains live.';
COMMENT ON COLUMN operations.runs.scheduled_at IS 'Logical occurrence instant used for idempotent run creation.';
COMMENT ON COLUMN operations.runs.created_at IS 'Actual run creation and monitoring-round ordering time.';
COMMENT ON COLUMN operations.runs.finished_at IS 'Completion or cancellation time.';
COMMENT ON TABLE operations.targets IS 'Durable per-site work with fenced leases and checkpointed attempts.';
COMMENT ON COLUMN operations.targets.id IS 'Stable target-work identifier.';
COMMENT ON COLUMN operations.targets.run_id IS 'Owning run.';
COMMENT ON COLUMN operations.targets.site_id IS 'Canonical site captured for this operation.';
COMMENT ON COLUMN operations.targets.feed_id IS 'RSS feed identifier; null for site monitoring or a deleted feed.';
COMMENT ON COLUMN operations.targets.kind IS 'Operation used to exclude concurrent work of the same kind.';
COMMENT ON COLUMN operations.targets.address IS 'Request address captured when the run was created.';
COMMENT ON COLUMN operations.targets.generation IS 'Address generation used to reject stale observations.';
COMMENT ON COLUMN operations.targets.access_scope IS 'Observed access scope at run creation; routing reads current stable scope.';
COMMENT ON COLUMN operations.targets.state IS 'Target execution lifecycle.';
COMMENT ON COLUMN operations.targets.lease_token IS 'Worker-only fencing token, invalidated on recovery or cancellation.';
COMMENT ON COLUMN operations.targets.lease_until IS 'Lease expiry extended by matching heartbeats.';
COMMENT ON COLUMN operations.targets.available_at IS 'Earliest dispatch time for request spacing.';
COMMENT ON COLUMN operations.targets.progress IS 'Persisted route and consumed attempt checkpoints for interruption recovery.';
COMMENT ON COLUMN operations.targets.result IS 'Completed path results without response bodies or internal diagnostics.';
COMMENT ON COLUMN operations.targets.finished_at IS 'Target completion or cancellation time.';
COMMENT ON TABLE operations.signing_policy IS 'Current outbound proof policy, editable only by SYS_ADMIN.';
COMMENT ON COLUMN operations.signing_policy.id IS 'Singleton key fixed to one.';
COMMENT ON COLUMN operations.signing_policy.enabled IS 'Whether every outbound request must carry a fresh proof.';
COMMENT ON COLUMN operations.signing_policy.carrier IS 'Proof in both UA and header, or in the header alone.';
COMMENT ON COLUMN operations.signing_policy.ttl_seconds IS 'Proof validity duration from 60 to 300 seconds.';
COMMENT ON TABLE operations.signing_keys IS 'Independent platform keys; private key bytes are AES-256-GCM ciphertext.';
COMMENT ON COLUMN operations.signing_keys.id IS 'Public key identifier and authenticated encryption binding.';
COMMENT ON COLUMN operations.signing_keys.platform IS 'Cloudflare CF or EdgeOne EO signing identity.';
COMMENT ON COLUMN operations.signing_keys.public_key IS 'Public ES256 JWK retained for historical verification.';
COMMENT ON COLUMN operations.signing_keys.encrypted_private_key IS 'Encrypted private key; master encryption key is never stored in the database.';
COMMENT ON COLUMN operations.signing_keys.state IS 'Generated, published, active, retired or revoked key lifecycle.';
COMMENT ON COLUMN operations.signing_keys.created_at IS 'Key generation time.';
COMMENT ON COLUMN operations.signing_keys.published_at IS 'First public key publication time.';
COMMENT ON COLUMN operations.signing_keys.revoked_at IS 'Revocation time; restored keys are always revoked.';
COMMENT ON TABLE operations.signing_events IS 'Append-only application audit of key and signing-policy operations.';
COMMENT ON COLUMN operations.signing_events.id IS 'Signing-operation audit identifier.';
COMMENT ON COLUMN operations.signing_events.key_id IS 'Affected key, or null for policy changes.';
COMMENT ON COLUMN operations.signing_events.actor_id IS 'Administrator who performed the operation.';
COMMENT ON COLUMN operations.signing_events.action IS 'Stable operation name without secret material.';
COMMENT ON COLUMN operations.signing_events.created_at IS 'Audit event creation time.';
COMMENT ON TABLE operations.request_attempts IS 'Persisted per-hop authorization and delivery receipts; uncertain delivery remains unknown.';
COMMENT ON COLUMN operations.request_attempts.id IS 'Dispatch reservation identifier.';
COMMENT ON COLUMN operations.request_attempts.target_id IS 'Associated task target.';
COMMENT ON COLUMN operations.request_attempts.platform IS 'Platform executing the request.';
COMMENT ON COLUMN operations.request_attempts.signed IS 'Whether live signing policy issued a proof for this dispatch.';
COMMENT ON COLUMN operations.request_attempts.created_at IS 'Dispatch reservation time.';
COMMENT ON TABLE operations.feed_documents IS 'Validators for successfully parsed feed documents.';
COMMENT ON COLUMN operations.feed_documents.feed_id IS 'Canonical feed whose document was validated.';
COMMENT ON COLUMN operations.feed_documents.etag IS 'Validated document ETag for conditional requests.';
COMMENT ON COLUMN operations.feed_documents.last_modified IS 'Validated document Last-Modified header.';
COMMENT ON COLUMN operations.feed_documents.checked_at IS 'Last successful document storage time.';
COMMENT ON TABLE operations.feed_items IS 'Validated and deduplicated RSS, Atom and JSON Feed items.';
COMMENT ON COLUMN operations.feed_items.feed_id IS 'Canonical source feed.';
COMMENT ON COLUMN operations.feed_items.item_key IS 'SHA-256 of the source item identifier or canonical item URL.';
COMMENT ON COLUMN operations.feed_items.title IS 'Source item title.';
COMMENT ON COLUMN operations.feed_items.url IS 'Validated absolute HTTP or HTTPS item URL.';
COMMENT ON COLUMN operations.feed_items.published IS 'Source publication value retained without guessing its timezone.';
COMMENT ON TABLE operations.route_state IS 'Atomic alternating route preference for ALL or unknown access scope.';
COMMENT ON COLUMN operations.route_state.site_id IS 'Site whose next preference is stored.';
COMMENT ON COLUMN operations.route_state.operation IS 'Independent RSS or ownership-verification routing sequence.';
COMMENT ON COLUMN operations.route_state.next_cf IS 'Whether the next unspecified-scope request prefers Cloudflare.';

COMMENT ON SCHEMA operations IS 'API-owned durable tasks, request authorization, signing and operational statistics.';
COMMENT ON TABLE operations.request_configs IS 'Reusable revisioned ordinary request parameters with explicit unsigned approval.';
COMMENT ON TABLE operations.task_settings IS 'Default request configuration bindings for business event tasks.';
COMMENT ON TABLE operations.workers IS 'Protocol-fenced ephemeral worker instances; restarts use fresh identities.';
COMMENT ON TABLE operations.task_events IS 'Task management audit without credentials, bodies or internal diagnostics.';
COMMENT ON TABLE operations.task_idempotency IS 'Payload-bound mutation receipts retained after detail cleanup.';
COMMENT ON TABLE operations.platform_backoff IS 'Response-driven shared platform backoff and single recovery probe.';
COMMENT ON TABLE operations.request_statistics IS 'Idempotent cumulative request facts independent of detail retention.';
-- +goose StatementBegin
DO $comments$
DECLARE item record;
BEGIN
 FOR item IN SELECT * FROM (VALUES
 ('request_configs','id','Stable request configuration identity.'),
 ('request_configs','name','Administrator-facing configuration name.'),
 ('request_configs','purpose','Validated business purpose.'),
 ('request_configs','enabled','Whether new tasks may reference the configuration.'),
 ('request_configs','archived','Archive flag independent of enabled state.'),
 ('request_configs','revision','Optimistic edit revision.'),
 ('request_configs','parameters','Validated ordinary request settings; never secret material.'),
 ('request_configs','created_at','Creation instant.'),
 ('task_settings','id','Singleton settings identity.'),
 ('task_settings','verification_config_id','Default HTTP claim verification request configuration.'),
 ('workers','id','Fresh process instance identity.'),
 ('workers','protocol','Accepted internal task protocol version.'),
 ('workers','version','Reported executable version.'),
 ('workers','registered_at','First registration instant.'),
 ('workers','heartbeat_at','Latest API-confirmed instance heartbeat.'),
 ('workers','draining','Stop dispatching schedules and claiming new work.'),
 ('schedules','request_config_id','Reusable request settings resolved when a run is created.'),
 ('schedules','archived','Archived schedules cannot trigger.'),
 ('runs','source','Scheduled, manual, event, retry or rerun trigger.'),
 ('runs','actor_id','Authenticated initiating user, or null for system triggers.'),
 ('runs','request_config_id','Original reusable request configuration identity.'),
 ('runs','root_run_id','First run in the retry chain.'),
 ('runs','parent_run_id','Immediate source of this retry or rerun.'),
 ('runs','retry_number','Monotone sequence within the root retry chain.'),
 ('runs','started_at','First target execution instant; queue time is excluded.'),
 ('runs','paused_at','Fully paused instant, or null.'),
 ('runs','paused_ms','Accumulated fully paused duration in milliseconds.'),
 ('runs','timeout_seconds','Execution deadline including backoff and pause draining.'),
 ('runs','next_dispatch_at','Independent serialized dispatch pacing state.'),
 ('runs','archived','Independent history archive flag.'),
 ('runs','cleaned_at','Detail cleanup tombstone instant.'),
 ('runs','reason','Stable completion or control reason code.'),
 ('targets','worker_id','Instance owning the active fenced lease.'),
 ('targets','claim_id','Claim application captured by HTTP verification.'),
 ('targets','challenge_digest','Challenge identity digest; never raw verification token.'),
 ('targets','claim_snapshot','Immutable claim decision inputs without proof material.'),
 ('targets','round_id','Logical sample identity retained through automatic recovery.'),
 ('targets','sample_started_at','Actual first observation start, never run creation time.'),
 ('targets','recoveries','Consumed process-failure recoveries; maximum three takeovers.'),
 ('targets','reason','Stable target execution reason.'),
 ('targets','usable','Whether confirmed usable business evidence was committed.'),
 ('signing_policy','revision','Live proof policy generation fencing cached permits.'),
 ('request_attempts','request_key','Worker-generated idempotency identity for one hop.'),
 ('request_attempts','worker_id','Instance identity attached to the original authorization.'),
 ('request_attempts','lease_token','Private authorization fence retained for idempotent receipts.'),
 ('request_attempts','kind','Business task type for independent accounting.'),
 ('request_attempts','url','Canonical authorized URL; admin projections redact query and fragment.'),
 ('request_attempts','method','Authorized request method.'),
 ('request_attempts','logical_retry','Consumed network retry identity.'),
 ('request_attempts','hop','Redirect sequence within one logical attempt.'),
 ('request_attempts','parent_id','Preceding redirect hop identity.'),
 ('request_attempts','key_id','Signing key selected before sending.'),
 ('request_attempts','policy_revision','Live policy generation at authorization.'),
 ('request_attempts','state','Authorized, reported sent, received or delivery unknown.'),
 ('request_attempts','expires_at','Dispatch permit expiry.'),
 ('request_attempts','sent_at','Worker-confirmed pre-send instant.'),
 ('request_attempts','received_at','API-confirmed result receipt instant.'),
 ('request_attempts','response_digest','Hash binding idempotent result submission.'),
 ('request_attempts','response_meta','Sanitized hop result metadata without response body.'),
 ('request_attempts','accepted','Business effects and receipt committed in the same transaction.'),
 ('task_events','id','Stable audit identity.'),
 ('task_events','run_id','Related run when applicable.'),
 ('task_events','actor_id','Authenticated administrator or null system actor.'),
 ('task_events','entity_id','Affected schedule, config, worker or run identity.'),
 ('task_events','action','Stable audited operation.'),
 ('task_events','outcome','Stable operation result.'),
 ('task_events','created_at','Audit instant.'),
 ('task_idempotency','scope','Actor and operation namespace.'),
 ('task_idempotency','key','Explicit mutation identity.'),
 ('task_idempotency','digest','Canonical payload digest rejecting conflicting reuse.'),
 ('task_idempotency','result','Original safe operation receipt.'),
 ('task_idempotency','created_at','Receipt instant.'),
 ('platform_backoff','platform','Executor platform.'),
 ('platform_backoff','retry_at','Earliest shared recovery probe time.'),
 ('platform_backoff','reason','Stable platform response reason.'),
 ('platform_backoff','stage','Bounded exponential backoff stage.'),
 ('platform_backoff','probe_id','Attempt holding the single recovery probe.'),
 ('platform_backoff','probe_until','Probe reservation expiry.'),
 ('request_statistics','attempt_id','Unique accounting identity without detail foreign key.'),
 ('request_statistics','platform','Authorized executor platform.'),
 ('request_statistics','kind','Business operation type.'),
 ('request_statistics','created_at','Authorization accounting instant.'),
 ('request_statistics','authorized','Authorization was committed.'),
 ('request_statistics','sent','Worker reported pre-send stage.'),
 ('request_statistics','received','A result was received.'),
 ('request_statistics','succeeded','Execution returned a usable target response.'),
 ('request_statistics','failed','Confirmed execution failure.'),
 ('request_statistics','unknown','Unconfirmed delivery after process or lease loss.'),
 ('request_statistics','duration_ms','Reported bounded execution duration.'),
 ('request_statistics','retries','Logical retry number.'),
 ('request_statistics','redirects','Redirect hop number.'),
 ('request_statistics','fallback','Alternative persisted business route used.')
 ) AS values_list(table_name,column_name,description)
 LOOP
  EXECUTE format('COMMENT ON COLUMN operations.%I.%I IS %L',item.table_name,item.column_name,item.description);
 END LOOP;
END $comments$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Forward-only migration; restore the previous backup.'; END $$;
-- +goose StatementEnd

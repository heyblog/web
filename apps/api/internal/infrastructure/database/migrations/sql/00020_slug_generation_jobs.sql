-- +goose Up
CREATE TABLE directory.slug_generation_jobs (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 task identifier.
    owner_id uuid REFERENCES identity.users(id) ON DELETE SET NULL, -- Initiating actor, cleared on account deletion; workers recheck current authorization.
    identity_ip_hash text NOT NULL CHECK (identity_ip_hash ~ '^[a-f0-9]{64}$'), -- Digest of the trusted initiating client IP used for shared request limits.
    model_id text NOT NULL, -- Frozen server-selected provider model.
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','paused','ready','cancelled','completed')), -- Generation and application lifecycle state.
    revision bigint NOT NULL DEFAULT 1 CHECK (revision>0), -- Optimistic concurrency version of task previews and controls.
    items jsonb NOT NULL CHECK (jsonb_typeof(items)='array' AND jsonb_array_length(items) BETWEEN 1 AND 500), -- Typed internal snapshots including tag revision, generated candidates, and per-item results.
    pause_code text NOT NULL DEFAULT '', -- Stable safe reason code for a paused job.
    resume_after timestamptz, -- Earliest retry time reported by the shared budget guard.
    lease_token text NOT NULL DEFAULT '', -- Worker fencing token; never returned through HTTP.
    lease_until timestamptz, -- Worker lease expiration; dispatched items are not automatically retried after expiration.
    created_at timestamptz NOT NULL DEFAULT now(), -- Task creation time.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Most recent task transition time.
    CONSTRAINT slug_generation_jobs_model_check CHECK (btrim(model_id)<>'')
);
CREATE UNIQUE INDEX slug_generation_jobs_active_owner_idx ON directory.slug_generation_jobs(owner_id)
    WHERE status IN ('queued','running','paused');
CREATE INDEX slug_generation_jobs_claim_idx ON directory.slug_generation_jobs(status,created_at);
GRANT SELECT,INSERT,UPDATE ON directory.slug_generation_jobs TO api_runtime;
COMMENT ON TABLE directory.slug_generation_jobs IS 'Durable authorized slug previews; applying selected items changes dictionary slugs atomically.';
COMMENT ON COLUMN directory.slug_generation_jobs.id IS 'UUIDv7 task identifier.';
COMMENT ON COLUMN directory.slug_generation_jobs.owner_id IS 'Initiating actor, cleared on account deletion; workers recheck current authorization.';
COMMENT ON COLUMN directory.slug_generation_jobs.identity_ip_hash IS 'Digest of the trusted initiating client IP used for shared request limits.';
COMMENT ON COLUMN directory.slug_generation_jobs.model_id IS 'Frozen server-selected provider model.';
COMMENT ON COLUMN directory.slug_generation_jobs.status IS 'Generation and application lifecycle state.';
COMMENT ON COLUMN directory.slug_generation_jobs.revision IS 'Optimistic concurrency version of task previews and controls.';
COMMENT ON COLUMN directory.slug_generation_jobs.items IS 'Typed internal snapshots including tag revision, generated candidates, and per-item results.';
COMMENT ON COLUMN directory.slug_generation_jobs.pause_code IS 'Stable safe reason code for a paused job.';
COMMENT ON COLUMN directory.slug_generation_jobs.resume_after IS 'Earliest retry time reported by the shared budget guard.';
COMMENT ON COLUMN directory.slug_generation_jobs.lease_token IS 'Worker fencing token; never returned through HTTP.';
COMMENT ON COLUMN directory.slug_generation_jobs.lease_until IS 'Worker lease expiration; dispatched items are not automatically retried after expiration.';
COMMENT ON COLUMN directory.slug_generation_jobs.created_at IS 'Task creation time.';
COMMENT ON COLUMN directory.slug_generation_jobs.updated_at IS 'Most recent task transition time.';

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
    RAISE EXCEPTION 'Migration 00020 is forward-only; restore the pre-upgrade database backup to downgrade safely.';
END $$;
-- +goose StatementEnd

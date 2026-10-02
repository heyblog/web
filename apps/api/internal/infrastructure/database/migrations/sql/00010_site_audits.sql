-- +goose Up
INSERT INTO directory.software_components (
    name,
    normalized_name,
    description,
    homepage_url,
    repository_url,
    is_open_source,
    is_enabled
) VALUES (
    '其他',
    '其他',
    '站点程序未列入目录时使用此选项。',
    NULL,
    NULL,
    false,
    true
)
ON CONFLICT (normalized_name) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    is_enabled = true;

CREATE TABLE directory.site_audits (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 site audit primary key.
    lookup_secret_hash bytea NOT NULL, -- SHA-256 hash of the anonymous high-entropy lookup credential.
    action text NOT NULL, -- Requested lifecycle action: CREATE, UPDATE, DELETE, or RESTORE.
    status text NOT NULL DEFAULT 'PENDING', -- Review state: PENDING, APPROVED, or REJECTED.
    site_id uuid REFERENCES directory.sites(id) ON DELETE RESTRICT, -- Target site; absent only while a CREATE request is pending.
    base_revision bigint, -- Site revision captured when a non-CREATE request was submitted.
    base_snapshot jsonb, -- Immutable aggregate snapshot captured before the requested change.
    proposed_snapshot jsonb NOT NULL, -- Immutable aggregate snapshot containing the requested result.
    review_draft_snapshot jsonb, -- Latest normalized reviewer correction saved before a final decision.
    review_draft_revision bigint NOT NULL DEFAULT 0, -- Monotonic version used to prevent concurrent reviewer draft overwrites.
    review_draft_updated_by uuid REFERENCES identity.users(id) ON DELETE SET NULL, -- Administrator who last saved or discarded the reviewer draft.
    review_draft_updated_at timestamptz, -- Time the reviewer draft was last saved or discarded.
    final_snapshot jsonb, -- Aggregate snapshot applied after an approved review and optional correction.
    request_reason text NOT NULL, -- Submitter explanation for the requested action.
    submitter_name text, -- Optional submitter display name used only for review context.
    submitter_email text, -- Optional submitter mailbox used only for decision notification.
    notify_by_email boolean NOT NULL DEFAULT false, -- Whether the submitter requested a decision email.
    reviewer_comment text, -- Decision note intentionally visible to the submitter.
    reviewed_by uuid REFERENCES identity.users(id) ON DELETE SET NULL, -- Administrator who completed the review.
    reviewed_at timestamptz, -- Time the review decision became final.
    created_at timestamptz NOT NULL DEFAULT now(), -- Time the anonymous request was created.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Last audit state update time maintained by trigger.
    CONSTRAINT site_audits_lookup_hash_check CHECK (octet_length(lookup_secret_hash) = 32),
    CONSTRAINT site_audits_lookup_hash_unique UNIQUE (lookup_secret_hash),
    CONSTRAINT site_audits_action_check CHECK (action IN ('CREATE', 'UPDATE', 'DELETE', 'RESTORE')),
    CONSTRAINT site_audits_status_check CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    CONSTRAINT site_audits_revision_check CHECK (
        (action = 'CREATE' AND base_revision IS NULL AND base_snapshot IS NULL)
        OR (action <> 'CREATE' AND site_id IS NOT NULL AND base_revision >= 1 AND jsonb_typeof(base_snapshot) = 'object')
    ),
    CONSTRAINT site_audits_create_site_check CHECK (
        action <> 'CREATE' OR status = 'PENDING' OR site_id IS NOT NULL
    ),
    CONSTRAINT site_audits_snapshot_check CHECK (
        jsonb_typeof(proposed_snapshot) = 'object'
        AND (review_draft_snapshot IS NULL OR (
            action IN ('CREATE', 'UPDATE')
            AND jsonb_typeof(review_draft_snapshot) = 'object'
        ))
        AND (final_snapshot IS NULL OR jsonb_typeof(final_snapshot) = 'object')
    ),
    CONSTRAINT site_audits_review_draft_check CHECK (
        review_draft_revision >= 0
        AND (
            (review_draft_revision = 0 AND review_draft_snapshot IS NULL AND review_draft_updated_at IS NULL)
            OR (review_draft_revision > 0 AND review_draft_updated_at IS NOT NULL)
        )
    ),
    CONSTRAINT site_audits_reason_check CHECK (action = 'CREATE' OR btrim(request_reason) <> ''),
    CONSTRAINT site_audits_submitter_name_check CHECK (submitter_name IS NULL OR btrim(submitter_name) <> ''),
    CONSTRAINT site_audits_submitter_email_check CHECK (submitter_email IS NULL OR btrim(submitter_email) <> ''),
    CONSTRAINT site_audits_notification_check CHECK (NOT notify_by_email OR submitter_email IS NOT NULL),
    CONSTRAINT site_audits_review_state_check CHECK (
        (status = 'PENDING' AND final_snapshot IS NULL AND reviewer_comment IS NULL AND reviewed_by IS NULL AND reviewed_at IS NULL)
        OR (status = 'APPROVED' AND final_snapshot IS NOT NULL AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
        OR (status = 'REJECTED' AND final_snapshot IS NULL AND btrim(reviewer_comment) <> '' AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
    )
);

CREATE INDEX site_audits_status_created_idx
ON directory.site_audits (status, created_at DESC, id DESC);
CREATE INDEX site_audits_action_status_idx
ON directory.site_audits (action, status, created_at DESC);
CREATE INDEX site_audits_site_created_idx
ON directory.site_audits (site_id, created_at DESC) WHERE site_id IS NOT NULL;
CREATE UNIQUE INDEX site_audits_pending_site_unique_idx
ON directory.site_audits (site_id) WHERE status = 'PENDING' AND site_id IS NOT NULL;
CREATE UNIQUE INDEX site_audits_pending_create_host_unique_idx
ON directory.site_audits ((proposed_snapshot ->> 'normalized_host'))
WHERE status = 'PENDING' AND action = 'CREATE';

-- +goose StatementBegin
CREATE FUNCTION directory.preserve_site_audit_submission()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.lookup_secret_hash IS DISTINCT FROM NEW.lookup_secret_hash
       OR OLD.action IS DISTINCT FROM NEW.action
       OR OLD.base_revision IS DISTINCT FROM NEW.base_revision
       OR OLD.base_snapshot IS DISTINCT FROM NEW.base_snapshot
       OR OLD.proposed_snapshot IS DISTINCT FROM NEW.proposed_snapshot
       OR OLD.request_reason IS DISTINCT FROM NEW.request_reason
       OR OLD.submitter_name IS DISTINCT FROM NEW.submitter_name
       OR OLD.submitter_email IS DISTINCT FROM NEW.submitter_email
       OR OLD.notify_by_email IS DISTINCT FROM NEW.notify_by_email
       OR OLD.created_at IS DISTINCT FROM NEW.created_at THEN
        RAISE EXCEPTION 'site audit submission fields are immutable';
    END IF;
    NEW.updated_at = clock_timestamp();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER site_audits_preserve_submission
BEFORE UPDATE ON directory.site_audits
FOR EACH ROW EXECUTE FUNCTION directory.preserve_site_audit_submission();

COMMENT ON FUNCTION directory.preserve_site_audit_submission() IS 'Preserves submitted audit evidence while maintaining the audit update time.';
COMMENT ON TABLE directory.site_audits IS 'Anonymous site lifecycle requests with immutable aggregate evidence and reviewed outcomes.';
COMMENT ON TRIGGER site_audits_preserve_submission ON directory.site_audits IS 'Prevents submitted audit evidence from changing during review.';
COMMENT ON COLUMN directory.site_audits.id IS 'UUIDv7 site audit primary key.';
COMMENT ON COLUMN directory.site_audits.lookup_secret_hash IS 'SHA-256 hash of the anonymous high-entropy lookup credential.';
COMMENT ON COLUMN directory.site_audits.action IS 'Requested lifecycle action: CREATE, UPDATE, DELETE, or RESTORE.';
COMMENT ON COLUMN directory.site_audits.status IS 'Review state: PENDING, APPROVED, or REJECTED.';
COMMENT ON COLUMN directory.site_audits.site_id IS 'Target site; absent only while a CREATE request is pending.';
COMMENT ON COLUMN directory.site_audits.base_revision IS 'Site revision captured when a non-CREATE request was submitted.';
COMMENT ON COLUMN directory.site_audits.base_snapshot IS 'Immutable aggregate snapshot captured before the requested change.';
COMMENT ON COLUMN directory.site_audits.proposed_snapshot IS 'Immutable aggregate snapshot containing the requested result.';
COMMENT ON COLUMN directory.site_audits.review_draft_snapshot IS 'Latest normalized reviewer correction saved before a final decision.';
COMMENT ON COLUMN directory.site_audits.review_draft_revision IS 'Monotonic version used to prevent concurrent reviewer draft overwrites.';
COMMENT ON COLUMN directory.site_audits.review_draft_updated_by IS 'Administrator who last saved or discarded the reviewer draft.';
COMMENT ON COLUMN directory.site_audits.review_draft_updated_at IS 'Time the reviewer draft was last saved or discarded.';
COMMENT ON COLUMN directory.site_audits.final_snapshot IS 'Aggregate snapshot applied after an approved review and optional correction.';
COMMENT ON COLUMN directory.site_audits.request_reason IS 'Submitter explanation for the requested action.';
COMMENT ON COLUMN directory.site_audits.submitter_name IS 'Optional submitter display name used only for review context.';
COMMENT ON COLUMN directory.site_audits.submitter_email IS 'Optional submitter mailbox used only for decision notification.';
COMMENT ON COLUMN directory.site_audits.notify_by_email IS 'Whether the submitter requested a decision email.';
COMMENT ON COLUMN directory.site_audits.reviewer_comment IS 'Decision note intentionally visible to the submitter.';
COMMENT ON COLUMN directory.site_audits.reviewed_by IS 'Administrator who completed the review.';
COMMENT ON COLUMN directory.site_audits.reviewed_at IS 'Time the review decision became final.';
COMMENT ON COLUMN directory.site_audits.created_at IS 'Time the anonymous request was created.';
COMMENT ON COLUMN directory.site_audits.updated_at IS 'Last audit state update time maintained by trigger.';

GRANT SELECT, INSERT, UPDATE ON directory.site_audits TO api_runtime;

-- +goose Down
REVOKE ALL ON directory.site_audits FROM api_runtime;
DROP TRIGGER site_audits_preserve_submission ON directory.site_audits;
DROP FUNCTION directory.preserve_site_audit_submission();
DROP TABLE directory.site_audits;
UPDATE directory.software_components
SET is_enabled = false
WHERE normalized_name = '其他';

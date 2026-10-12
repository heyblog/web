-- +goose Up

SET LOCAL check_function_bodies = false;

-- +goose StatementBegin

CREATE FUNCTION directory.preserve_site_audit_submission() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE anonymizing boolean;
BEGIN
    anonymizing := OLD.submitter_user_id IS NOT NULL AND NEW.submitter_user_id IS NULL
       AND NEW.submitter_name IS NULL AND NEW.submitter_email IS NULL AND NOT NEW.notify_by_email
       AND NOT EXISTS (SELECT 1 FROM identity.users WHERE id = OLD.submitter_user_id AND deleted_at IS NULL);
    IF OLD.lookup_secret_hash IS DISTINCT FROM NEW.lookup_secret_hash
       OR OLD.action IS DISTINCT FROM NEW.action
       OR OLD.base_revision IS DISTINCT FROM NEW.base_revision
       OR OLD.base_snapshot IS DISTINCT FROM NEW.base_snapshot
       OR OLD.proposed_snapshot IS DISTINCT FROM NEW.proposed_snapshot
       OR OLD.request_reason IS DISTINCT FROM NEW.request_reason
       OR (NOT anonymizing AND OLD.submitter_name IS DISTINCT FROM NEW.submitter_name)
       OR (NOT anonymizing AND OLD.submitter_email IS DISTINCT FROM NEW.submitter_email)
       OR (NOT anonymizing AND OLD.notify_by_email IS DISTINCT FROM NEW.notify_by_email)
       OR OLD.created_at IS DISTINCT FROM NEW.created_at THEN
        RAISE EXCEPTION 'site audit submission fields are immutable';
    END IF;
    NEW.updated_at = clock_timestamp();
    RETURN NEW;
END;
$$;

-- +goose StatementEnd

CREATE TABLE directory.site_audits (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 site audit primary key.
    lookup_secret_hash bytea NOT NULL, -- SHA-256 hash of the anonymous high-entropy lookup credential.
    action text NOT NULL, -- Requested lifecycle action: CREATE, UPDATE, DELETE, or RESTORE.
    status text DEFAULT 'PENDING'::text NOT NULL, -- Review state: PENDING, APPROVED, or REJECTED.
    site_id uuid, -- Target site; a CREATE request requires a site only after approval.
    base_revision bigint, -- Site revision captured when a non-CREATE request was submitted.
    base_snapshot jsonb, -- Immutable aggregate snapshot captured before the requested change.
    proposed_snapshot jsonb NOT NULL, -- Immutable aggregate snapshot containing the requested result.
    review_draft_snapshot jsonb, -- Latest normalized reviewer correction saved before a final decision.
    review_draft_revision bigint DEFAULT 0 NOT NULL, -- Monotonic version used to prevent concurrent reviewer draft overwrites.
    review_draft_updated_by uuid, -- Administrator who last saved or discarded the reviewer draft.
    review_draft_updated_at timestamp with time zone, -- Time the reviewer draft was last saved or discarded.
    final_snapshot jsonb, -- Aggregate snapshot applied after an approved review and optional correction.
    request_reason text NOT NULL, -- Submitter explanation for the requested action.
    submitter_name text, -- Optional submitter display name used only for review context.
    submitter_email text, -- Optional submitter mailbox used only for decision notification.
    notify_by_email boolean DEFAULT false NOT NULL, -- Whether the submitter requested a decision email.
    reviewer_comment text, -- Decision note intentionally visible to the submitter.
    reviewed_by uuid, -- Administrator who completed the review.
    reviewed_at timestamp with time zone, -- Time the review decision became final.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Time the anonymous request was created.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last audit state update time maintained by trigger.
    submitter_user_id uuid, -- Authenticated submitting account; null for anonymous submissions or deleted accounts.
    source_channel text DEFAULT 'ANONYMOUS'::text NOT NULL, -- Server-assigned submission provenance channel.
    source_site_id uuid, -- Authenticated owner source site for updates and friend recommendations.
    ownership_id uuid, -- Immutable ownership generation captured when the owner submitted the request.
    CONSTRAINT site_audits_action_check CHECK ((action = ANY (ARRAY['CREATE'::text, 'UPDATE'::text, 'DELETE'::text, 'RESTORE'::text]))),
    CONSTRAINT site_audits_create_site_check CHECK (((action <> 'CREATE'::text) OR (status <> 'APPROVED'::text) OR (site_id IS NOT NULL))),
    CONSTRAINT site_audits_lookup_hash_check CHECK ((octet_length(lookup_secret_hash) = 32)),
    CONSTRAINT site_audits_notification_check CHECK (((NOT notify_by_email) OR (submitter_email IS NOT NULL))),
    CONSTRAINT site_audits_owner_context_check CHECK ((((source_channel = ANY (ARRAY['ANONYMOUS'::text, 'ACCOUNT_SUBMISSION'::text])) AND (source_site_id IS NULL) AND (ownership_id IS NULL)) OR ((source_channel = ANY (ARRAY['OWNER_UPDATE'::text, 'OWNER_FRIEND_LINK'::text])) AND (source_site_id IS NOT NULL) AND (ownership_id IS NOT NULL)))),
    CONSTRAINT site_audits_reason_check CHECK (((action = 'CREATE'::text) OR (btrim(request_reason) <> ''::text))),
    CONSTRAINT site_audits_review_draft_check CHECK (((review_draft_revision >= 0) AND (((review_draft_revision = 0) AND (review_draft_snapshot IS NULL) AND (review_draft_updated_at IS NULL)) OR ((review_draft_revision > 0) AND (review_draft_updated_at IS NOT NULL))))),
    CONSTRAINT site_audits_review_state_check CHECK ((((status = 'PENDING'::text) AND (final_snapshot IS NULL) AND (reviewer_comment IS NULL) AND (reviewed_by IS NULL) AND (reviewed_at IS NULL)) OR ((status = 'APPROVED'::text) AND (final_snapshot IS NOT NULL) AND (reviewed_by IS NOT NULL) AND (reviewed_at IS NOT NULL)) OR ((status = 'REJECTED'::text) AND (final_snapshot IS NULL) AND (btrim(reviewer_comment) <> ''::text) AND (reviewed_by IS NOT NULL) AND (reviewed_at IS NOT NULL)))),
    CONSTRAINT site_audits_revision_check CHECK ((((action = 'CREATE'::text) AND (base_revision IS NULL) AND (base_snapshot IS NULL)) OR ((action <> 'CREATE'::text) AND (site_id IS NOT NULL) AND (base_revision >= 1) AND (jsonb_typeof(base_snapshot) = 'object'::text)))),
    CONSTRAINT site_audits_snapshot_check CHECK (((jsonb_typeof(proposed_snapshot) = 'object'::text) AND ((review_draft_snapshot IS NULL) OR ((action = ANY (ARRAY['CREATE'::text, 'UPDATE'::text])) AND (jsonb_typeof(review_draft_snapshot) = 'object'::text))) AND ((final_snapshot IS NULL) OR (jsonb_typeof(final_snapshot) = 'object'::text)))),
    CONSTRAINT site_audits_source_channel_check CHECK ((source_channel = ANY (ARRAY['ANONYMOUS'::text, 'ACCOUNT_SUBMISSION'::text, 'OWNER_UPDATE'::text, 'OWNER_FRIEND_LINK'::text]))),
    CONSTRAINT site_audits_status_check CHECK ((status = ANY (ARRAY['PENDING'::text, 'APPROVED'::text, 'REJECTED'::text]))),
    CONSTRAINT site_audits_submitter_email_check CHECK (((submitter_email IS NULL) OR (btrim(submitter_email) <> ''::text))),
    CONSTRAINT site_audits_submitter_name_check CHECK (((submitter_name IS NULL) OR (btrim(submitter_name) <> ''::text)))
,
    CONSTRAINT site_audits_lookup_hash_unique UNIQUE (lookup_secret_hash)
,
    CONSTRAINT site_audits_pkey PRIMARY KEY (id)
);

CREATE INDEX site_audits_action_status_idx ON directory.site_audits USING btree (action, status, created_at DESC);

CREATE UNIQUE INDEX site_audits_pending_create_host_unique_idx ON directory.site_audits USING btree (((proposed_snapshot ->> 'normalized_host'::text))) WHERE ((status = 'PENDING'::text) AND (action = 'CREATE'::text));

CREATE UNIQUE INDEX site_audits_pending_site_unique_idx ON directory.site_audits USING btree (site_id) WHERE ((status = 'PENDING'::text) AND (site_id IS NOT NULL));

CREATE INDEX site_audits_site_created_idx ON directory.site_audits USING btree (site_id, created_at DESC) WHERE (site_id IS NOT NULL);

CREATE INDEX site_audits_status_created_idx ON directory.site_audits USING btree (status, created_at DESC, id DESC);

CREATE INDEX site_audits_submitter_idx ON directory.site_audits USING btree (submitter_user_id, created_at DESC);

CREATE TRIGGER site_audits_preserve_submission BEFORE UPDATE ON directory.site_audits FOR EACH ROW EXECUTE FUNCTION directory.preserve_site_audit_submission();

COMMENT ON FUNCTION directory.preserve_site_audit_submission() IS 'Preserves submitted audit evidence except terminal account anonymization, while maintaining update time.';

COMMENT ON TABLE directory.site_audits IS 'Anonymous site lifecycle requests with immutable aggregate evidence and reviewed outcomes.';

COMMENT ON COLUMN directory.site_audits.id IS 'UUIDv7 site audit primary key.';

COMMENT ON COLUMN directory.site_audits.lookup_secret_hash IS 'SHA-256 hash of the anonymous high-entropy lookup credential.';

COMMENT ON COLUMN directory.site_audits.action IS 'Requested lifecycle action: CREATE, UPDATE, DELETE, or RESTORE.';

COMMENT ON COLUMN directory.site_audits.status IS 'Review state: PENDING, APPROVED, or REJECTED.';

COMMENT ON COLUMN directory.site_audits.site_id IS 'Target site; a CREATE request requires a site only after approval.';

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

COMMENT ON COLUMN directory.site_audits.submitter_user_id IS 'Authenticated submitting account; null for anonymous submissions or deleted accounts.';

COMMENT ON COLUMN directory.site_audits.source_channel IS 'Server-assigned submission provenance channel.';

COMMENT ON COLUMN directory.site_audits.source_site_id IS 'Authenticated owner source site for updates and friend recommendations.';

COMMENT ON COLUMN directory.site_audits.ownership_id IS 'Immutable ownership generation captured when the owner submitted the request.';

COMMENT ON TRIGGER site_audits_preserve_submission ON directory.site_audits IS 'Prevents submitted audit evidence from changing during review.';

GRANT SELECT,INSERT,UPDATE ON TABLE directory.site_audits TO api_runtime;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The consolidated baseline is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd

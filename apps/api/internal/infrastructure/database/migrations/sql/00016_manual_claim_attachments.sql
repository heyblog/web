-- +goose Up

SELECT directory.backup_check_schema();

ALTER TABLE directory.site_claims
    ADD COLUMN reviewed_at timestamp with time zone; -- Time the claim left the pending review lifecycle; null while pending.

COMMENT ON COLUMN directory.site_claims.reviewed_at IS 'Time the claim left the pending review lifecycle; null while pending.';

CREATE TABLE directory.site_claim_attachments (
    id uuid DEFAULT uuidv7() NOT NULL, -- Attachment identifier used by the manual claim API.
    claim_id uuid, -- Bound manual claim; null until a ready upload is submitted.
    site_id uuid NOT NULL, -- Registered site selected when the upload session is created.
    user_id uuid, -- Uploading account; nulled during terminal account anonymization.
    address text NOT NULL, -- Exact normalized address selected for the evidence.
    filename text NOT NULL, -- Original basename shown to reviewers.
    content_type text NOT NULL, -- Validated media type: PNG, JPEG, or PDF.
    size bigint NOT NULL, -- Validated byte size; PDF is capped at 10 MiB and images at 5 MiB.
    sha256 text, -- Digest of the validated object bytes.
    storage_key text NOT NULL, -- Private storage key; never exposed as a permanent public URL.
    storage_identity text NOT NULL, -- COS origin that owns the evidence; prevents cleanup in another bucket.
    upload_cleaned_at timestamp with time zone, -- Time the expired temporary upload key was fully removed.
    status text DEFAULT 'UPLOADING'::text NOT NULL, -- Upload and retention lifecycle: UPLOADING, READY, BOUND, CLEANUP_PENDING, ARCHIVED, or DELETED.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Upload session creation time.
    completed_at timestamp with time zone, -- Time content validation completed.
    bound_at timestamp with time zone, -- Time the attachment was atomically bound to a claim.
    cleanup_requested_at timestamp with time zone, -- Time cleanup was requested for cancellation, removal, or account deletion.
    archive_at timestamp with time zone, -- Time the object may transition to archive storage after case closure.
    purge_at timestamp with time zone, -- Time the evidence object may be deleted after case closure.
    CONSTRAINT site_claim_attachments_pkey PRIMARY KEY (id),
    CONSTRAINT site_claim_attachments_claim_id_fkey FOREIGN KEY (claim_id) REFERENCES directory.site_claims(id) ON DELETE RESTRICT,
    CONSTRAINT site_claim_attachments_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id) ON DELETE RESTRICT,
    CONSTRAINT site_claim_attachments_user_id_fkey FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE SET NULL,
    CONSTRAINT site_claim_attachments_content_type_check CHECK (content_type = ANY (ARRAY['image/png'::text, 'image/jpeg'::text, 'application/pdf'::text])),
    CONSTRAINT site_claim_attachments_size_check CHECK (size > 0 AND ((content_type = 'application/pdf'::text AND size <= (10 * 1024 * 1024)) OR (content_type <> 'application/pdf'::text AND size <= (5 * 1024 * 1024)))),
    CONSTRAINT site_claim_attachments_sha256_check CHECK (sha256 IS NULL OR sha256 ~ '^[0-9a-f]{64}$'::text),
    CONSTRAINT site_claim_attachments_status_check CHECK (status = ANY (ARRAY['UPLOADING'::text, 'READY'::text, 'BOUND'::text, 'CLEANUP_PENDING'::text, 'ARCHIVED'::text, 'DELETED'::text])),
    CONSTRAINT site_claim_attachments_address_check CHECK (address ~ '^https?://'::text),
    CONSTRAINT site_claim_attachments_claim_binding_check CHECK ((status IN ('UPLOADING', 'READY') AND claim_id IS NULL) OR (status = 'BOUND' AND claim_id IS NOT NULL) OR status IN ('CLEANUP_PENDING', 'ARCHIVED', 'DELETED'))
);

COMMENT ON COLUMN directory.site_claim_attachments.storage_identity IS 'COS origin that owns the evidence; prevents cleanup in another bucket.';
COMMENT ON COLUMN directory.site_claim_attachments.upload_cleaned_at IS 'Time the expired temporary upload key was fully removed.';

CREATE INDEX site_claim_attachments_claim_idx ON directory.site_claim_attachments (claim_id, created_at, id);
CREATE INDEX site_claim_attachments_owner_idx ON directory.site_claim_attachments (user_id, status, created_at DESC);
CREATE INDEX site_claim_attachments_cleanup_idx ON directory.site_claim_attachments (status, archive_at, purge_at) WHERE status IN ('CLEANUP_PENDING', 'BOUND', 'ARCHIVED');

COMMENT ON TABLE directory.site_claim_attachments IS 'Private evidence metadata for manual site verification; object bytes remain in dedicated external storage.';
COMMENT ON COLUMN directory.site_claim_attachments.id IS 'Attachment identifier used by the manual claim API.';
COMMENT ON COLUMN directory.site_claim_attachments.claim_id IS 'Bound manual claim; null until a ready upload is submitted.';
COMMENT ON COLUMN directory.site_claim_attachments.site_id IS 'Registered site selected when the upload session is created.';
COMMENT ON COLUMN directory.site_claim_attachments.user_id IS 'Uploading account; nulled during terminal account anonymization.';
COMMENT ON COLUMN directory.site_claim_attachments.address IS 'Exact normalized address selected for the evidence.';
COMMENT ON COLUMN directory.site_claim_attachments.filename IS 'Original basename shown to reviewers.';
COMMENT ON COLUMN directory.site_claim_attachments.content_type IS 'Validated media type: PNG, JPEG, or PDF.';
COMMENT ON COLUMN directory.site_claim_attachments.size IS 'Validated byte size; PDF is capped at 10 MiB and images at 5 MiB.';
COMMENT ON COLUMN directory.site_claim_attachments.sha256 IS 'Digest of the validated object bytes.';
COMMENT ON COLUMN directory.site_claim_attachments.storage_key IS 'Private storage key; never exposed as a permanent public URL.';
COMMENT ON COLUMN directory.site_claim_attachments.status IS 'Upload and retention lifecycle: UPLOADING, READY, BOUND, CLEANUP_PENDING, ARCHIVED, or DELETED.';
COMMENT ON COLUMN directory.site_claim_attachments.created_at IS 'Upload session creation time.';
COMMENT ON COLUMN directory.site_claim_attachments.completed_at IS 'Time content validation completed.';
COMMENT ON COLUMN directory.site_claim_attachments.bound_at IS 'Time the attachment was atomically bound to a claim.';
COMMENT ON COLUMN directory.site_claim_attachments.cleanup_requested_at IS 'Time cleanup was requested for cancellation, removal, or account deletion.';
COMMENT ON COLUMN directory.site_claim_attachments.archive_at IS 'Time the object may transition to archive storage after case closure.';
COMMENT ON COLUMN directory.site_claim_attachments.purge_at IS 'Time the evidence object may be deleted after case closure.';

-- +goose StatementBegin
CREATE FUNCTION directory.schedule_site_claim_attachment_cleanup() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
        UPDATE directory.site_claim_attachments
           SET status = 'CLEANUP_PENDING', cleanup_requested_at = clock_timestamp(), user_id = NULL
         WHERE user_id = NEW.id AND status <> 'DELETED';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.schedule_cancelled_site_claim_attachment_cleanup() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF OLD.status IN ('PENDING', 'VERIFIED') AND NEW.status = 'CANCELLED' THEN
        UPDATE directory.site_claim_attachments
           SET status = 'CLEANUP_PENDING', cleanup_requested_at = clock_timestamp()
         WHERE claim_id = NEW.id AND status <> 'DELETED';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.schedule_closed_site_claim_attachment_retention() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF OLD.status = 'PENDING' AND NEW.status IN ('VERIFIED', 'REJECTED') THEN
        UPDATE directory.site_claim_attachments
           SET archive_at = COALESCE(NEW.reviewed_at, clock_timestamp()) + interval '30 days',
               purge_at = COALESCE(NEW.reviewed_at, clock_timestamp()) + interval '365 days'
         WHERE claim_id = NEW.id AND status = 'BOUND';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.schedule_removed_site_claim_attachment_cleanup() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF OLD.visibility <> 'REMOVED' AND NEW.visibility = 'REMOVED' THEN
        UPDATE directory.site_claim_attachments
           SET status = 'CLEANUP_PENDING', cleanup_requested_at = clock_timestamp()
         WHERE site_id = NEW.id AND status <> 'DELETED';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER users_site_claim_attachment_cleanup
    AFTER UPDATE OF deleted_at ON identity.users
    FOR EACH ROW EXECUTE FUNCTION directory.schedule_site_claim_attachment_cleanup();

CREATE TRIGGER site_claim_attachment_cleanup_on_cancel
    AFTER UPDATE OF status ON directory.site_claims
    FOR EACH ROW EXECUTE FUNCTION directory.schedule_cancelled_site_claim_attachment_cleanup();

CREATE TRIGGER site_claim_attachment_retention_on_close
    AFTER UPDATE OF status ON directory.site_claims
    FOR EACH ROW EXECUTE FUNCTION directory.schedule_closed_site_claim_attachment_retention();

CREATE TRIGGER site_claim_attachment_cleanup_on_remove
    AFTER UPDATE OF visibility ON directory.sites
    FOR EACH ROW EXECUTE FUNCTION directory.schedule_removed_site_claim_attachment_cleanup();

COMMENT ON FUNCTION directory.schedule_site_claim_attachment_cleanup() IS 'Schedules private claim evidence cleanup when an account reaches terminal deletion.';
COMMENT ON FUNCTION directory.schedule_cancelled_site_claim_attachment_cleanup() IS 'Schedules private claim evidence cleanup when a pending claim is cancelled.';
COMMENT ON FUNCTION directory.schedule_closed_site_claim_attachment_retention() IS 'Schedules archive and purge deadlines when a manual claim is decided.';
COMMENT ON TRIGGER users_site_claim_attachment_cleanup ON identity.users IS 'Removes account ownership of evidence and schedules object cleanup after deletion.';
COMMENT ON TRIGGER site_claim_attachment_cleanup_on_cancel ON directory.site_claims IS 'Schedules evidence cleanup for cancelled claims and removed sites.';
COMMENT ON TRIGGER site_claim_attachment_retention_on_close ON directory.site_claims IS 'Sets 30-day archive and 365-day purge deadlines for decided claims.';
COMMENT ON FUNCTION directory.schedule_removed_site_claim_attachment_cleanup() IS 'Schedules evidence cleanup when a site becomes removed.';
COMMENT ON TRIGGER site_claim_attachment_cleanup_on_remove ON directory.sites IS 'Schedules evidence cleanup when a site becomes removed.';

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE directory.site_claim_attachments TO api_runtime;

-- Capture the exact new schema only after checking the previous baseline above.
-- Subsequent exports/restores continue to reject any unrelated schema drift.
-- +goose StatementBegin
DO $migration$
DECLARE expected text;
BEGIN
 SELECT md5(string_agg(n.nspname||'.'||c.relname||':'||a.attname||':'||a.atttypid::regtype::text||':'||a.attgenerated::text,',' ORDER BY n.nspname,c.relname,a.attnum)) INTO expected
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
 WHERE n.nspname IN ('identity','directory','content') AND c.relkind IN ('r','p');
 EXECUTE format($definition$
 CREATE OR REPLACE FUNCTION directory.backup_check_schema() RETURNS void
 LANGUAGE plpgsql STABLE SET search_path TO 'pg_catalog' AS $body$
 DECLARE signature text;
 BEGIN
 SELECT md5(string_agg(n.nspname||'.'||c.relname||':'||a.attname||':'||a.atttypid::regtype::text||':'||a.attgenerated::text,',' ORDER BY n.nspname,c.relname,a.attnum)) INTO signature
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
 WHERE n.nspname IN ('identity','directory','content') AND c.relkind IN ('r','p');
 IF signature <> %L THEN
 RAISE EXCEPTION 'unsupported backup schema' USING ERRCODE='22023';
 END IF;
 END $body$;
 $definition$, expected);
END $migration$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_tables() RETURNS text[]
    LANGUAGE sql IMMUTABLE
    AS $$
 SELECT ARRAY[
 'identity.users','identity.oauth_identities','identity.email_verification_codes','identity.password_reset_tokens','identity.user_management_permissions',
 'identity.api_clients','identity.api_client_scopes','identity.api_keys',
 'directory.tags','directory.tag_cascades','directory.tag_identity_aliases','directory.tag_assignment_archive',
 'directory.software_components','directory.software_component_dependencies','directory.site_sources',
 'directory.sites','directory.site_feeds','directory.site_icons','directory.site_resources','directory.site_software_components','directory.site_origins','directory.site_tags',
 'directory.site_claims','directory.site_claim_attachments','directory.site_ownerships','directory.site_ownership_events',
 'directory.site_audits','directory.owner_friend_link_requests','directory.site_metrics','directory.site_metric_events',
 'content.articles','content.article_tags','content.announcements','content.announcement_revisions']::text[];
$$;
-- +goose StatementEnd

COMMENT ON FUNCTION directory.backup_check_schema() IS 'Checks the exact schema including manual claim attachments; external COS bytes are excluded.';
COMMENT ON FUNCTION directory.backup_tables() IS 'Includes manual claim attachment metadata; private COS evidence bytes remain external.';

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The consolidated baseline is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd

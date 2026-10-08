-- +goose Up
ALTER TABLE directory.site_audits
    ADD COLUMN submitter_user_id uuid REFERENCES identity.users(id) ON DELETE SET NULL, -- Authenticated submitting account; null for anonymous submissions or deleted accounts.
    ADD COLUMN source_channel text NOT NULL DEFAULT 'ANONYMOUS', -- Server-assigned submission provenance channel.
    ADD COLUMN source_site_id uuid REFERENCES directory.sites(id) ON DELETE RESTRICT, -- Authenticated owner source site for updates and friend recommendations.
    ADD COLUMN ownership_id uuid, -- Immutable ownership generation captured when the owner submitted the request.
    ADD CONSTRAINT site_audits_source_channel_check CHECK (source_channel IN ('ANONYMOUS', 'ACCOUNT_SUBMISSION', 'OWNER_UPDATE', 'OWNER_FRIEND_LINK')),
    ADD CONSTRAINT site_audits_owner_context_check CHECK (
        (source_channel IN ('ANONYMOUS', 'ACCOUNT_SUBMISSION') AND source_site_id IS NULL AND ownership_id IS NULL)
        OR (source_channel IN ('OWNER_UPDATE', 'OWNER_FRIEND_LINK') AND source_site_id IS NOT NULL AND ownership_id IS NOT NULL)
    );

CREATE INDEX site_audits_submitter_idx ON directory.site_audits (submitter_user_id, created_at DESC);

CREATE TABLE directory.owner_friend_link_requests (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 friend request identifier.
    audit_id uuid NOT NULL REFERENCES directory.site_audits(id) ON DELETE RESTRICT, -- Existing CREATE audit for the requested target.
    source_site_id uuid NOT NULL REFERENCES directory.sites(id) ON DELETE RESTRICT, -- Site recommending the target.
    user_id uuid REFERENCES identity.users(id) ON DELETE SET NULL, -- Recommending account; null after deletion.
    ownership_id uuid NOT NULL, -- Ownership generation required when the link is applied.
    status text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPLIED', 'CANCELLED', 'REJECTED')), -- Request lifecycle: PENDING, APPLIED, CANCELLED, or REJECTED.
    created_at timestamptz NOT NULL DEFAULT now(), -- Request submission time.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Last lifecycle update time.
    UNIQUE (audit_id, source_site_id, ownership_id)
);
CREATE INDEX owner_friend_requests_user_idx ON directory.owner_friend_link_requests (user_id, created_at DESC);
CREATE TRIGGER owner_friend_requests_touch BEFORE UPDATE ON directory.owner_friend_link_requests
FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

-- +goose StatementBegin
CREATE FUNCTION directory.validate_owner_audit_submission()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    owner_row directory.site_ownerships;
    current_url text;
    proposed_url text;
BEGIN
    IF NEW.source_channel NOT IN ('OWNER_UPDATE', 'OWNER_FRIEND_LINK') THEN RETURN NEW; END IF;
    SELECT scheme || '://' || normalized_host || base_path INTO current_url
      FROM directory.sites WHERE id = NEW.source_site_id AND visibility <> 'REMOVED' FOR UPDATE;
    SELECT * INTO owner_row FROM directory.site_ownerships WHERE id = NEW.ownership_id FOR UPDATE;
    IF current_url IS NULL OR owner_row.id IS NULL OR owner_row.site_id <> NEW.source_site_id
       OR owner_row.user_id IS DISTINCT FROM NEW.submitter_user_id OR owner_row.address <> current_url THEN
        RAISE EXCEPTION 'active site ownership is required' USING ERRCODE = 'P0001';
    END IF;
    IF NEW.source_channel = 'OWNER_UPDATE' THEN
        IF NEW.action <> 'UPDATE' OR NEW.site_id <> NEW.source_site_id THEN
            RAISE EXCEPTION 'invalid owner update target';
        END IF;
        proposed_url := (NEW.proposed_snapshot ->> 'scheme') || '://' || (NEW.proposed_snapshot ->> 'normalized_host') || (NEW.proposed_snapshot ->> 'base_path');
        IF proposed_url <> current_url AND NOT EXISTS (
            SELECT 1 FROM directory.site_claims WHERE site_id = NEW.site_id AND user_id = NEW.submitter_user_id
             AND address = proposed_url AND status = 'VERIFIED' AND consumed_at IS NULL
             AND created_at >= owner_row.created_at
        ) THEN RAISE EXCEPTION 'new site address verification is required'; END IF;
    ELSIF NEW.action <> 'CREATE' THEN
        RAISE EXCEPTION 'friend link submission must create a site';
    END IF;
    RETURN NEW;
END;
$$;

CREATE FUNCTION directory.apply_owner_audit_effects()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    request_row directory.owner_friend_link_requests;
    owner_row directory.site_ownerships;
    target_row directory.sites;
    final_url text;
    v_source_key text;
    v_source_id uuid;
BEGIN
    IF OLD.source_channel IS DISTINCT FROM NEW.source_channel
       OR OLD.source_site_id IS DISTINCT FROM NEW.source_site_id
       OR OLD.ownership_id IS DISTINCT FROM NEW.ownership_id
       OR (OLD.submitter_user_id IS DISTINCT FROM NEW.submitter_user_id AND (NEW.submitter_user_id IS NOT NULL OR EXISTS (SELECT 1 FROM identity.users WHERE id = OLD.submitter_user_id AND deleted_at IS NULL))) THEN
        RAISE EXCEPTION 'audit provenance is immutable';
    END IF;
    IF OLD.status <> 'PENDING' OR NEW.status = 'PENDING' THEN RETURN NEW; END IF;
    IF NEW.status = 'REJECTED' THEN
        UPDATE directory.owner_friend_link_requests SET status = 'REJECTED' WHERE audit_id = NEW.id AND status = 'PENDING';
        RETURN NEW;
    END IF;
    -- Let the existing approval constraint reject a missing target before applying effects.
    IF NEW.site_id IS NULL THEN RETURN NEW; END IF;
    SELECT * INTO STRICT target_row FROM directory.sites WHERE id = NEW.site_id;
    final_url := target_row.scheme || '://' || target_row.normalized_host || target_row.base_path;
    IF NEW.source_channel = 'OWNER_UPDATE' THEN
        PERFORM directory.apply_verified_site_address(NEW.site_id, NEW.submitter_user_id, final_url, NEW.ownership_id);
    END IF;
    IF NEW.action = 'CREATE' AND NEW.source_channel <> 'ANONYMOUS' THEN
        v_source_key := CASE WHEN NEW.source_channel = 'OWNER_FRIEND_LINK' THEN 'OWNER_FRIEND_LINK' ELSE 'ACCOUNT_SUBMISSION' END;
        INSERT INTO directory.site_sources (source_key, name) VALUES (v_source_key, CASE WHEN v_source_key = 'OWNER_FRIEND_LINK' THEN '站主友链推荐' ELSE '用户提交' END)
        ON CONFLICT (source_key) DO UPDATE SET is_enabled = true RETURNING id INTO v_source_id;
        INSERT INTO directory.site_origins (site_id, source_id, external_reference, metadata)
        VALUES (NEW.site_id, v_source_id, NEW.id::text, jsonb_build_object('channel', NEW.source_channel, 'user_id', NEW.submitter_user_id, 'source_site_id', NEW.source_site_id, 'audit_id', NEW.id))
        ON CONFLICT (site_id, source_id) DO NOTHING;
    END IF;
    FOR request_row IN SELECT * FROM directory.owner_friend_link_requests WHERE audit_id = NEW.id AND status = 'PENDING' ORDER BY source_site_id FOR UPDATE LOOP
        PERFORM 1 FROM directory.sites WHERE id = request_row.source_site_id FOR UPDATE;
        SELECT o.* INTO owner_row FROM directory.site_ownerships o JOIN directory.sites s ON s.id = o.site_id
         WHERE o.id = request_row.ownership_id AND o.user_id = request_row.user_id
           AND s.visibility <> 'REMOVED' AND o.address = s.scheme || '://' || s.normalized_host || s.base_path FOR UPDATE OF o;
        IF owner_row.id IS NULL OR owner_row.site_id = NEW.site_id THEN
            UPDATE directory.owner_friend_link_requests SET status = 'CANCELLED' WHERE id = request_row.id;
        ELSE
            PERFORM directory.upsert_registered_friend_link(request_row.source_site_id, final_url, target_row.normalized_host, 'ACTIVE');
            INSERT INTO directory.site_sources (source_key, name) VALUES ('OWNER_FRIEND_LINK', '站主友链推荐')
            ON CONFLICT (source_key) DO UPDATE SET is_enabled = true RETURNING id INTO v_source_id;
            INSERT INTO directory.site_origins (site_id, source_id, external_reference, metadata)
            VALUES (NEW.site_id, v_source_id, NEW.id::text, jsonb_build_object('channel', 'OWNER_FRIEND_LINK', 'recommendations', jsonb_build_array(jsonb_build_object('user_id', request_row.user_id, 'source_site_id', request_row.source_site_id, 'audit_id', NEW.id))))
            ON CONFLICT (site_id, source_id) DO UPDATE SET metadata = directory.site_origins.metadata || jsonb_build_object('recommendations', COALESCE(directory.site_origins.metadata -> 'recommendations', '[]'::jsonb) || (EXCLUDED.metadata -> 'recommendations'));
            UPDATE directory.owner_friend_link_requests SET status = 'APPLIED' WHERE id = request_row.id;
        END IF;
    END LOOP;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER site_audits_validate_owner BEFORE INSERT ON directory.site_audits
FOR EACH ROW EXECUTE FUNCTION directory.validate_owner_audit_submission();
CREATE TRIGGER site_audits_apply_owner_effects BEFORE UPDATE ON directory.site_audits
FOR EACH ROW EXECUTE FUNCTION directory.apply_owner_audit_effects();

COMMENT ON COLUMN directory.site_audits.submitter_user_id IS 'Authenticated submitting account; null for anonymous submissions or deleted accounts.';
COMMENT ON COLUMN directory.site_audits.source_channel IS 'Server-assigned submission provenance channel.';
COMMENT ON COLUMN directory.site_audits.source_site_id IS 'Authenticated owner source site for updates and friend recommendations.';
COMMENT ON COLUMN directory.site_audits.ownership_id IS 'Immutable ownership generation captured when the owner submitted the request.';
COMMENT ON TABLE directory.owner_friend_link_requests IS 'Deferred owner friend links applied atomically when the target site is approved.';
COMMENT ON COLUMN directory.owner_friend_link_requests.id IS 'UUIDv7 friend request identifier.';
COMMENT ON COLUMN directory.owner_friend_link_requests.audit_id IS 'Existing CREATE audit for the requested target.';
COMMENT ON COLUMN directory.owner_friend_link_requests.source_site_id IS 'Site recommending the target.';
COMMENT ON COLUMN directory.owner_friend_link_requests.user_id IS 'Recommending account; null after deletion.';
COMMENT ON COLUMN directory.owner_friend_link_requests.ownership_id IS 'Ownership generation required when the link is applied.';
COMMENT ON COLUMN directory.owner_friend_link_requests.status IS 'Request lifecycle: PENDING, APPLIED, CANCELLED, or REJECTED.';
COMMENT ON COLUMN directory.owner_friend_link_requests.created_at IS 'Request submission time.';
COMMENT ON COLUMN directory.owner_friend_link_requests.updated_at IS 'Last lifecycle update time.';
COMMENT ON FUNCTION directory.validate_owner_audit_submission() IS 'Locks and validates active owner authority and address proof at submission.';
COMMENT ON FUNCTION directory.apply_owner_audit_effects() IS 'Preserves provenance and applies address proof, origins and pending graph links within approval transactions.';
COMMENT ON TRIGGER site_audits_validate_owner ON directory.site_audits IS 'Requires current ownership for authenticated owner submissions.';
COMMENT ON TRIGGER site_audits_apply_owner_effects ON directory.site_audits IS 'Atomically completes owner-related audit effects.';
COMMENT ON TRIGGER owner_friend_requests_touch ON directory.owner_friend_link_requests IS 'Maintains request update time.';
GRANT SELECT, INSERT, UPDATE ON directory.owner_friend_link_requests TO api_runtime;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.preserve_site_audit_submission()
RETURNS trigger
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

CREATE FUNCTION directory.anonymize_owner_submissions()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
        -- Serialize with decisions before the later ownership-release trigger locks sites.
        PERFORM 1 FROM directory.site_audits a WHERE a.submitter_user_id = NEW.id OR EXISTS (
            SELECT 1 FROM directory.owner_friend_link_requests r WHERE r.audit_id = a.id AND r.user_id = NEW.id
        ) ORDER BY a.id FOR UPDATE;
        UPDATE directory.site_audits SET submitter_user_id = NULL, submitter_name = NULL,
            submitter_email = NULL, notify_by_email = false WHERE submitter_user_id = NEW.id;
        UPDATE directory.owner_friend_link_requests SET user_id = NULL,
            status = CASE WHEN status = 'PENDING' THEN 'CANCELLED' ELSE status END WHERE user_id = NEW.id;
        UPDATE directory.site_origins SET metadata =
            (CASE WHEN metadata ->> 'user_id' = NEW.id::text THEN metadata - 'user_id' ELSE metadata END)
            || CASE WHEN metadata ? 'recommendations' THEN jsonb_build_object('recommendations',
                COALESCE((SELECT jsonb_agg(CASE WHEN item ->> 'user_id' = NEW.id::text THEN item - 'user_id' ELSE item END)
                FROM jsonb_array_elements(metadata -> 'recommendations') item), '[]'::jsonb)) ELSE '{}'::jsonb END
        WHERE metadata ->> 'user_id' = NEW.id::text OR EXISTS (
            SELECT 1 FROM jsonb_array_elements(COALESCE(metadata -> 'recommendations', '[]'::jsonb)) item WHERE item ->> 'user_id' = NEW.id::text
        );
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER users_00_anonymize_owner_submissions AFTER UPDATE OF deleted_at ON identity.users
FOR EACH ROW EXECUTE FUNCTION directory.anonymize_owner_submissions();
COMMENT ON FUNCTION directory.anonymize_owner_submissions() IS 'Removes account identity and contact from authenticated submissions, deferred recommendations and origin metadata at terminal deletion.';
COMMENT ON TRIGGER users_00_anonymize_owner_submissions ON identity.users IS 'Anonymizes authenticated directory submission history before ownership release.';
COMMENT ON FUNCTION directory.preserve_site_audit_submission() IS 'Preserves submitted audit evidence except terminal account anonymization, while maintaining update time.';

-- +goose Down
DROP TRIGGER users_00_anonymize_owner_submissions ON identity.users;
DROP FUNCTION directory.anonymize_owner_submissions();
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.preserve_site_audit_submission()
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

DROP TRIGGER site_audits_apply_owner_effects ON directory.site_audits;
DROP TRIGGER site_audits_validate_owner ON directory.site_audits;
DROP FUNCTION directory.apply_owner_audit_effects();
DROP FUNCTION directory.validate_owner_audit_submission();
DROP TABLE directory.owner_friend_link_requests;
ALTER TABLE directory.site_audits DROP CONSTRAINT site_audits_owner_context_check, DROP CONSTRAINT site_audits_source_channel_check,
    DROP COLUMN ownership_id, DROP COLUMN source_site_id, DROP COLUMN source_channel, DROP COLUMN submitter_user_id;

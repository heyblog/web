-- +goose Up

ALTER TABLE directory.site_claims
    ADD COLUMN email_change_id uuid, -- The email change whose site control this claim proves, retained after the request is removed.
    ADD COLUMN email_change_ownership_id uuid, -- The ownership generation that must still control the registered address when this proof is applied.
    ADD CONSTRAINT site_claims_email_change_proof_check CHECK (
        (email_change_id IS NULL AND email_change_ownership_id IS NULL)
        OR (email_change_id IS NOT NULL AND email_change_ownership_id IS NOT NULL
            AND (status <> 'VERIFIED' OR consumed_at IS NOT NULL))
    );

COMMENT ON COLUMN directory.site_claims.email_change_id IS 'The email change whose site control this claim proves, retained after the request is removed.';
COMMENT ON COLUMN directory.site_claims.email_change_ownership_id IS 'The ownership generation that must still control the registered address when this proof is applied.';

CREATE INDEX site_claims_email_change_idx ON directory.site_claims (email_change_id, site_id, created_at DESC)
    WHERE email_change_id IS NOT NULL;
COMMENT ON INDEX directory.site_claims_email_change_idx IS 'Finds scoped claims for a pending email change and supports expiry cleanup.';

-- +goose StatementBegin
CREATE FUNCTION directory.invalidate_email_change_site_proofs() RETURNS trigger
    LANGUAGE plpgsql SET search_path TO 'pg_catalog'
AS $$
BEGIN
    UPDATE directory.site_claims c
    SET status = 'CANCELLED', reviewed_at = clock_timestamp()
    FROM identity.email_change_requests r
    WHERE c.site_id = NEW.id AND c.email_change_id = r.id
      AND c.status IN ('PENDING', 'VERIFIED');
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER sites_invalidate_email_change_proofs
    AFTER UPDATE OF scheme, normalized_host, base_path, visibility, blog_status ON directory.sites
    FOR EACH ROW WHEN (
        OLD.scheme IS DISTINCT FROM NEW.scheme OR OLD.normalized_host IS DISTINCT FROM NEW.normalized_host
        OR OLD.base_path IS DISTINCT FROM NEW.base_path OR (OLD.visibility <> 'REMOVED' AND NEW.visibility = 'REMOVED')
        OR (OLD.blog_status = 'BLOG' AND NEW.blog_status <> 'BLOG')
    ) EXECUTE FUNCTION directory.invalidate_email_change_site_proofs();

COMMENT ON FUNCTION directory.invalidate_email_change_site_proofs() IS 'Invalidates in-progress email-change proofs when the site is moved or removed.';
COMMENT ON TRIGGER sites_invalidate_email_change_proofs ON directory.sites IS 'Cancels in-progress email-change proofs when the registered address, visibility or blog status invalidates them.';
REVOKE ALL ON FUNCTION directory.invalidate_email_change_site_proofs() FROM PUBLIC;

-- The caller already holds the user and request locks. Site decisions take
-- the user lock before site locks when granting ownership.
-- +goose StatementBegin
CREATE FUNCTION directory.complete_email_change_site_proofs(p_user uuid, p_request uuid) RETURNS boolean
    LANGUAGE plpgsql SET search_path TO 'pg_catalog'
AS $$
BEGIN
    PERFORM 1 FROM directory.sites s JOIN directory.site_ownerships o ON o.site_id = s.id
    WHERE o.user_id = p_user ORDER BY s.id FOR UPDATE OF s;

    IF NOT EXISTS (
        SELECT 1 FROM identity.email_change_requests r
        WHERE r.user_id = p_user AND r.id = p_request AND r.expires_at > clock_timestamp()
    ) THEN
        RETURN false;
    END IF;

    IF EXISTS (
        SELECT 1 FROM directory.sites s JOIN directory.site_ownerships o ON o.site_id = s.id
        WHERE o.user_id = p_user AND s.visibility <> 'REMOVED' AND s.blog_status = 'BLOG'
        AND NOT EXISTS (
            SELECT 1 FROM directory.site_claims c
            WHERE c.email_change_id = p_request AND c.email_change_ownership_id = o.id
            AND c.site_id = s.id AND c.user_id = p_user AND c.status = 'VERIFIED'
            AND c.address = s.scheme || '://' || s.normalized_host || s.base_path
        )
    ) THEN
        RETURN false;
    END IF;

    WITH current_proofs AS (
        SELECT o.id AS ownership_id, s.id AS site_id, proof.id AS claim_id, proof.address
        FROM directory.site_ownerships o JOIN directory.sites s ON s.id = o.site_id
        CROSS JOIN LATERAL (
            SELECT c.id, c.address FROM directory.site_claims c
            WHERE c.email_change_id = p_request AND c.email_change_ownership_id = o.id
            AND c.site_id = s.id AND c.user_id = p_user AND c.status = 'VERIFIED'
            AND c.address = s.scheme || '://' || s.normalized_host || s.base_path
            ORDER BY c.verified_at DESC, c.id DESC LIMIT 1
        ) proof
        WHERE o.user_id = p_user AND s.visibility <> 'REMOVED' AND s.blog_status = 'BLOG'
    ), refreshed AS (
        UPDATE directory.site_ownerships o
        SET address = p.address, verified_claim_id = p.claim_id, revision = o.revision + 1, updated_at = clock_timestamp()
        FROM current_proofs p WHERE o.id = p.ownership_id
        RETURNING o.site_id
    )
    INSERT INTO directory.site_ownership_events (site_id, user_id, actor_id, action, reason)
    SELECT site_id, p_user, p_user, 'VERIFIED', 'Site control reverified for an account email change'
    FROM refreshed;
    RETURN true;
END;
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION directory.complete_email_change_site_proofs(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION directory.complete_email_change_site_proofs(uuid, uuid) TO api_runtime;
COMMENT ON FUNCTION directory.complete_email_change_site_proofs(uuid, uuid) IS 'Checks every current site and installs verified proofs within the email change transaction.';

-- Transient email change requests are excluded from the export allowlist.
-- The schema guard still checks the exact database shape, including that table.
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
            IF signature IS DISTINCT FROM %L THEN
                RAISE EXCEPTION 'unsupported backup schema' USING ERRCODE='22023';
            END IF;
        END
        $body$;
    $definition$, expected);
END
$migration$;
-- +goose StatementEnd

COMMENT ON FUNCTION directory.backup_check_schema() IS 'Checks the exact schema including transient email change requests excluded from backup.';

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_restore_finish(p_schema_version integer) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog'
    AS $$
BEGIN
    IF p_schema_version IS NULL OR p_schema_version NOT BETWEEN 3 AND 8 THEN
        RAISE EXCEPTION 'unsupported backup schema' USING ERRCODE='22023';
    END IF;

    -- The existing finish verifies the restore guard and flushes deferred FK events.
    PERFORM directory.backup_restore_finish();
    IF p_schema_version < 7 THEN
        -- Match migration 17: only an approved CREATE or UPDATE establishes BLOG.
        ALTER TABLE directory.sites DISABLE TRIGGER sites_touch_site;
        UPDATE directory.sites AS site
           SET blog_status = 'BLOG'
         WHERE site.blog_status = 'UNREVIEWED'
           AND EXISTS (
               SELECT 1 FROM directory.site_audits AS audit
                WHERE audit.site_id = site.id AND audit.status = 'APPROVED'
                  AND audit.action IN ('CREATE', 'UPDATE')
           );
        ALTER TABLE directory.sites ENABLE TRIGGER sites_touch_site;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The consolidated baseline is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd

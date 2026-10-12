-- +goose Up

SET LOCAL check_function_bodies = false;

-- +goose StatementBegin

CREATE FUNCTION directory.anonymize_owner_submissions() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
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

-- +goose StatementBegin

CREATE FUNCTION directory.apply_owner_audit_effects() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
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

-- +goose StatementBegin

CREATE FUNCTION directory.apply_verified_site_address(p_site uuid, p_user uuid, p_address text, p_ownership uuid) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE v_owner directory.site_ownerships%ROWTYPE; v_claim uuid;
BEGIN
 PERFORM 1 FROM directory.sites WHERE id=p_site AND visibility <> 'REMOVED' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'site unavailable' USING ERRCODE='23514',CONSTRAINT='site_ownership_changed'; END IF;
 SELECT * INTO v_owner FROM directory.site_ownerships WHERE site_id=p_site AND id=p_ownership AND user_id=p_user FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'site ownership changed' USING ERRCODE='23514',CONSTRAINT='site_ownership_changed'; END IF;
 IF v_owner.address=p_address THEN RETURN; END IF;
 SELECT id INTO v_claim FROM directory.site_claims WHERE site_id=p_site AND user_id=p_user AND address=p_address
 AND status='VERIFIED' AND consumed_at IS NULL AND created_at>=v_owner.created_at ORDER BY verified_at DESC LIMIT 1 FOR UPDATE;
 IF v_claim IS NULL THEN RAISE EXCEPTION 'matching verified address required' USING ERRCODE='23514',CONSTRAINT='verified_site_address_required'; END IF;
 UPDATE directory.site_claims SET consumed_at=now() WHERE id=v_claim;
 UPDATE directory.site_ownerships SET address=p_address,verified_claim_id=v_claim,revision=revision+1,updated_at=now() WHERE id=v_owner.id;
 INSERT INTO directory.site_ownership_events(site_id,user_id,actor_id,action,reason) VALUES(p_site,p_user,p_user,'ADDRESS_CHANGED','Approved site address update');
END;
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.list_owner_friend_links(p_source_site_id uuid, p_include_inactive boolean DEFAULT false) RETURNS TABLE(target_site_id uuid, target_host text, target_url text, link_status text, is_reciprocal boolean, created_at_ms bigint, updated_at_ms bigint)
    LANGUAGE plpgsql STABLE SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'ag_catalog', 'directory'
    AS $_$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM directory.sites
         WHERE id = p_source_site_id AND visibility <> 'REMOVED'
    ) THEN
        RETURN;
    END IF;

    RETURN QUERY EXECUTE format(
        $query$
        WITH graph_links AS (
            SELECT NULLIF(trim(both '"' FROM graph_target_id::text), 'null')::uuid AS resolved_site_id,
                   trim(both '"' FROM graph_target_host::text) AS resolved_host,
                   trim(both '"' FROM graph_target_url::text) AS resolved_url,
                   trim(both '"' FROM graph_status::text) AS resolved_status,
                   graph_reciprocal::text::boolean AS resolved_reciprocal,
                   graph_created::text::bigint AS resolved_created,
                   graph_updated::text::bigint AS resolved_updated
              FROM ag_catalog.cypher('directory_graph', $cypher$
                  MATCH (source:SiteRef {site_id: %s})-[edge:FRIEND_LINK]->(target:SiteRef)
                  OPTIONAL MATCH (target)-[reverse:FRIEND_LINK]->(source)
                  WHERE reverse.status = "ACTIVE"
                  RETURN target.site_id, target.normalized_host, edge.target_url, edge.status,
                         count(reverse) > 0, edge.created_at_ms, edge.updated_at_ms
              $cypher$) AS (
                  graph_target_id ag_catalog.agtype,
                  graph_target_host ag_catalog.agtype,
                  graph_target_url ag_catalog.agtype,
                  graph_status ag_catalog.agtype,
                  graph_reciprocal ag_catalog.agtype,
                  graph_created ag_catalog.agtype,
                  graph_updated ag_catalog.agtype
              )
        )
        SELECT link.resolved_site_id, link.resolved_host, link.resolved_url,
               link.resolved_status, link.resolved_reciprocal,
               link.resolved_created, link.resolved_updated
          FROM graph_links AS link
         WHERE (%L OR link.resolved_status = 'ACTIVE')
         ORDER BY link.resolved_host, link.resolved_url
        $query$,
        to_json(p_source_site_id::text)::text,
        p_include_inactive
    );
END;
$_$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.release_anonymized_site_ownership() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
  PERFORM 1 FROM directory.sites s WHERE s.id IN (SELECT site_id FROM directory.site_ownerships WHERE user_id=NEW.id UNION SELECT site_id FROM directory.site_claims WHERE user_id=NEW.id) ORDER BY s.id FOR UPDATE;
  DELETE FROM directory.site_ownerships WHERE user_id=NEW.id;
  UPDATE directory.site_claims SET user_id=NULL,evidence=NULL,evidence_url=NULL WHERE user_id=NEW.id AND method <> 'MANUAL';
  UPDATE directory.site_claims SET user_id=NULL,evidence='Applicant deleted',evidence_url=NULL WHERE user_id=NEW.id AND method='MANUAL';
  UPDATE directory.site_claims SET reviewed_by=NULL WHERE reviewed_by=NEW.id;
  UPDATE directory.site_ownership_events SET user_id=NULL,evidence=NULL WHERE user_id=NEW.id;
  UPDATE directory.site_ownership_events SET actor_id=NULL WHERE actor_id=NEW.id;
 END IF;
 RETURN NEW;
END;
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.remove_owner_friend_link(p_source_site_id uuid, p_target_host text) RETURNS boolean
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'ag_catalog', 'directory'
    AS $$
DECLARE
    existing_url text;
    event_ms bigint;
BEGIN
    PERFORM 1 FROM directory.sites WHERE id = p_source_site_id FOR UPDATE;
    PERFORM pg_advisory_xact_lock(hashtextextended('site-ref:' || p_target_host, 0));
    PERFORM pg_advisory_xact_lock(hashtextextended('friend-link:' || p_source_site_id::text || ':' || p_target_host, 0));
    SELECT target_url INTO existing_url FROM directory.list_owner_friend_links(p_source_site_id, true)
      WHERE target_host = p_target_host AND link_status = 'ACTIVE';
    IF existing_url IS NULL THEN RETURN false; END IF;
    event_ms := (extract(epoch FROM clock_timestamp()) * 1000)::bigint;
    PERFORM directory.merge_friend_link_graph(p_source_site_id, p_target_host, existing_url, 'INACTIVE', event_ms, event_ms);
    RETURN true;
END;
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.validate_owner_audit_submission() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    owner_row directory.site_ownerships;
    current_url text;
    proposed_url text;
BEGIN
    IF directory.backup_restore_is_active() THEN RETURN NEW; END IF;
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

-- +goose StatementEnd

CREATE TABLE directory.owner_friend_link_requests (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 friend request identifier.
    audit_id uuid NOT NULL, -- Existing CREATE audit for the requested target.
    source_site_id uuid NOT NULL, -- Site recommending the target.
    user_id uuid, -- Recommending account; null after deletion.
    ownership_id uuid NOT NULL, -- Ownership generation required when the link is applied.
    status text DEFAULT 'PENDING'::text NOT NULL, -- Request lifecycle: PENDING, APPLIED, CANCELLED, or REJECTED.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Request submission time.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last lifecycle update time.
    notify_by_email boolean DEFAULT false NOT NULL, -- Whether the recommending account wants an audit decision email.
    CONSTRAINT owner_friend_link_requests_status_check CHECK ((status = ANY (ARRAY['PENDING'::text, 'APPLIED'::text, 'CANCELLED'::text, 'REJECTED'::text])))
,
    CONSTRAINT owner_friend_link_requests_audit_id_source_site_id_ownershi_key UNIQUE (audit_id, source_site_id, ownership_id)
,
    CONSTRAINT owner_friend_link_requests_pkey PRIMARY KEY (id)
);

CREATE TABLE directory.site_claims (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 verification application identifier.
    site_id uuid NOT NULL, -- Registered site whose control is being proven.
    user_id uuid, -- Applicant, anonymized upon account deletion.
    address text NOT NULL, -- Exact normalized address covered by the proof.
    method text NOT NULL, -- DNS_TXT, META, FILE, or MANUAL verification.
    status text DEFAULT 'PENDING'::text NOT NULL, -- Verification lifecycle status.
    token_hash text, -- SHA256 digest; raw challenge tokens are never stored.
    expires_at timestamp with time zone, -- Automatic challenge deadline; manual requests have none.
    evidence text, -- Applicant public ownership evidence for human review.
    evidence_url text, -- Public evidence URL, displayed without fetching.
    reviewed_by uuid, -- Human reviewer, anonymized upon deletion.
    review_reason text, -- Human decision explanation.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Application creation time.
    verified_at timestamp with time zone, -- Successful verification time.
    consumed_at timestamp with time zone, -- Time proof was consumed by an ownership or address change.
    CONSTRAINT site_claims_address_check CHECK ((address ~ '^https?://'::text)),
    CONSTRAINT site_claims_check CHECK ((((method = 'MANUAL'::text) AND (token_hash IS NULL) AND (expires_at IS NULL) AND (evidence IS NOT NULL) AND (btrim(evidence) <> ''::text)) OR ((method <> 'MANUAL'::text) AND (token_hash ~ '^[0-9a-f]{64}$'::text) AND (expires_at > created_at)))),
    CONSTRAINT site_claims_check1 CHECK (((reviewed_by IS NULL) OR (reviewed_by <> user_id))),
    CONSTRAINT site_claims_method_check CHECK ((method = ANY (ARRAY['DNS_TXT'::text, 'META'::text, 'FILE'::text, 'MANUAL'::text]))),
    CONSTRAINT site_claims_status_check CHECK ((status = ANY (ARRAY['PENDING'::text, 'VERIFIED'::text, 'REJECTED'::text, 'CANCELLED'::text])))
,
    CONSTRAINT site_claims_pkey PRIMARY KEY (id)
);

CREATE TABLE directory.site_ownership_events (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 historical event identifier.
    site_id uuid NOT NULL, -- Affected registered site.
    user_id uuid, -- Affected owner, anonymized upon deletion.
    actor_id uuid, -- Acting user, anonymized upon deletion.
    action text NOT NULL, -- Ownership event category.
    reason text NOT NULL, -- Decision or operation explanation.
    evidence text, -- Administrative evidence, never fetched automatically.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Event creation time.
    CONSTRAINT site_ownership_events_action_check CHECK ((action = ANY (ARRAY['VERIFIED'::text, 'REVOKED'::text, 'REASSIGNED'::text, 'ADDRESS_CHANGED'::text])))
,
    CONSTRAINT site_ownership_events_pkey PRIMARY KEY (id)
);

CREATE TABLE directory.site_ownerships (
    id uuid DEFAULT uuidv7() NOT NULL, -- Ownership generation identifier; replacement invalidates pending edits.
    site_id uuid NOT NULL, -- Uniquely owned registered site.
    user_id uuid NOT NULL, -- Active owner; deletion releases ownership.
    address text NOT NULL, -- Exact verified canonical site address.
    verified_claim_id uuid, -- Proof establishing the current address, null for administrative assignment.
    revision bigint DEFAULT 1 NOT NULL, -- Monotonic revision within an ownership generation.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Start of this ownership generation.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Latest verified address change.
    CONSTRAINT site_ownerships_address_check CHECK ((address ~ '^https?://'::text)),
    CONSTRAINT site_ownerships_revision_check CHECK ((revision > 0))
,
    CONSTRAINT site_ownerships_pkey PRIMARY KEY (id)
,
    CONSTRAINT site_ownerships_site_id_key UNIQUE (site_id)
);

CREATE INDEX owner_friend_requests_user_idx ON directory.owner_friend_link_requests USING btree (user_id, created_at DESC);

CREATE INDEX site_claims_pending_idx ON directory.site_claims USING btree (created_at DESC) WHERE (status = 'PENDING'::text);

CREATE INDEX site_claims_site_user_idx ON directory.site_claims USING btree (site_id, user_id, created_at DESC);

COMMENT ON INDEX directory.site_claims_site_user_idx IS 'Supports site-specific pending application reads and atomic proof invalidation on ownership revocation.';

CREATE INDEX site_claims_user_idx ON directory.site_claims USING btree (user_id, created_at DESC);

CREATE INDEX site_ownership_events_site_idx ON directory.site_ownership_events USING btree (site_id, created_at DESC);

CREATE INDEX site_ownerships_user_idx ON directory.site_ownerships USING btree (user_id);

CREATE TRIGGER owner_friend_requests_touch BEFORE UPDATE ON directory.owner_friend_link_requests FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

CREATE TRIGGER site_audits_apply_owner_effects BEFORE UPDATE ON directory.site_audits FOR EACH ROW EXECUTE FUNCTION directory.apply_owner_audit_effects();

CREATE TRIGGER site_audits_validate_owner BEFORE INSERT ON directory.site_audits FOR EACH ROW EXECUTE FUNCTION directory.validate_owner_audit_submission();

CREATE TRIGGER users_00_anonymize_owner_submissions AFTER UPDATE OF deleted_at ON identity.users FOR EACH ROW EXECUTE FUNCTION directory.anonymize_owner_submissions();

CREATE TRIGGER users_release_site_ownership AFTER UPDATE OF deleted_at ON identity.users FOR EACH ROW EXECUTE FUNCTION directory.release_anonymized_site_ownership();

COMMENT ON FUNCTION directory.anonymize_owner_submissions() IS 'Removes account identity and contact from authenticated submissions, deferred recommendations and origin metadata at terminal deletion.';

COMMENT ON FUNCTION directory.apply_owner_audit_effects() IS 'Preserves provenance and applies address proof, origins and pending graph links within approval transactions.';

COMMENT ON FUNCTION directory.apply_verified_site_address(p_site uuid, p_user uuid, p_address text, p_ownership uuid) IS 'Consumes exact applicant/site/address proof and atomically changes a matching ownership generation.';

COMMENT ON FUNCTION directory.list_owner_friend_links(p_source_site_id uuid, p_include_inactive boolean) IS 'Reads all existing friend edges for authenticated owner workflows, including hidden sources and unavailable targets.';

COMMENT ON FUNCTION directory.release_anonymized_site_ownership() IS 'Releases ownership and anonymizes claim history on terminal account deletion.';

COMMENT ON FUNCTION directory.remove_owner_friend_link(p_source_site_id uuid, p_target_host text) IS 'Deactivates an existing registered or external friend edge without creating an unreviewed target.';

COMMENT ON FUNCTION directory.validate_owner_audit_submission() IS 'Validates live owner authority for submissions; the protected atomic restore scope preserves historical ownership generations.';

COMMENT ON TABLE directory.owner_friend_link_requests IS 'Deferred owner friend links applied atomically when the target site is approved.';

COMMENT ON COLUMN directory.owner_friend_link_requests.id IS 'UUIDv7 friend request identifier.';

COMMENT ON COLUMN directory.owner_friend_link_requests.audit_id IS 'Existing CREATE audit for the requested target.';

COMMENT ON COLUMN directory.owner_friend_link_requests.source_site_id IS 'Site recommending the target.';

COMMENT ON COLUMN directory.owner_friend_link_requests.user_id IS 'Recommending account; null after deletion.';

COMMENT ON COLUMN directory.owner_friend_link_requests.ownership_id IS 'Ownership generation required when the link is applied.';

COMMENT ON COLUMN directory.owner_friend_link_requests.status IS 'Request lifecycle: PENDING, APPLIED, CANCELLED, or REJECTED.';

COMMENT ON COLUMN directory.owner_friend_link_requests.created_at IS 'Request submission time.';

COMMENT ON COLUMN directory.owner_friend_link_requests.updated_at IS 'Last lifecycle update time.';

COMMENT ON COLUMN directory.owner_friend_link_requests.notify_by_email IS 'Whether the recommending account wants an audit decision email.';

COMMENT ON TABLE directory.site_claims IS 'Address-bound site control challenges and manual verification history.';

COMMENT ON COLUMN directory.site_claims.id IS 'UUIDv7 verification application identifier.';

COMMENT ON COLUMN directory.site_claims.site_id IS 'Registered site whose control is being proven.';

COMMENT ON COLUMN directory.site_claims.user_id IS 'Applicant, anonymized upon account deletion.';

COMMENT ON COLUMN directory.site_claims.address IS 'Exact normalized address covered by the proof.';

COMMENT ON COLUMN directory.site_claims.method IS 'DNS_TXT, META, FILE, or MANUAL verification.';

COMMENT ON COLUMN directory.site_claims.status IS 'Verification lifecycle status.';

COMMENT ON COLUMN directory.site_claims.token_hash IS 'SHA256 digest; raw challenge tokens are never stored.';

COMMENT ON COLUMN directory.site_claims.expires_at IS 'Automatic challenge deadline; manual requests have none.';

COMMENT ON COLUMN directory.site_claims.evidence IS 'Applicant public ownership evidence for human review.';

COMMENT ON COLUMN directory.site_claims.evidence_url IS 'Public evidence URL, displayed without fetching.';

COMMENT ON COLUMN directory.site_claims.reviewed_by IS 'Human reviewer, anonymized upon deletion.';

COMMENT ON COLUMN directory.site_claims.review_reason IS 'Human decision explanation.';

COMMENT ON COLUMN directory.site_claims.created_at IS 'Application creation time.';

COMMENT ON COLUMN directory.site_claims.verified_at IS 'Successful verification time.';

COMMENT ON COLUMN directory.site_claims.consumed_at IS 'Time proof was consumed by an ownership or address change.';

COMMENT ON TABLE directory.site_ownership_events IS 'Historical verification and administrative ownership decisions.';

COMMENT ON COLUMN directory.site_ownership_events.id IS 'UUIDv7 historical event identifier.';

COMMENT ON COLUMN directory.site_ownership_events.site_id IS 'Affected registered site.';

COMMENT ON COLUMN directory.site_ownership_events.user_id IS 'Affected owner, anonymized upon deletion.';

COMMENT ON COLUMN directory.site_ownership_events.actor_id IS 'Acting user, anonymized upon deletion.';

COMMENT ON COLUMN directory.site_ownership_events.action IS 'Ownership event category.';

COMMENT ON COLUMN directory.site_ownership_events.reason IS 'Decision or operation explanation.';

COMMENT ON COLUMN directory.site_ownership_events.evidence IS 'Administrative evidence, never fetched automatically.';

COMMENT ON COLUMN directory.site_ownership_events.created_at IS 'Event creation time.';

COMMENT ON TABLE directory.site_ownerships IS 'One currently effective owner per registered site.';

COMMENT ON COLUMN directory.site_ownerships.id IS 'Ownership generation identifier; replacement invalidates pending edits.';

COMMENT ON COLUMN directory.site_ownerships.site_id IS 'Uniquely owned registered site.';

COMMENT ON COLUMN directory.site_ownerships.user_id IS 'Active owner; deletion releases ownership.';

COMMENT ON COLUMN directory.site_ownerships.address IS 'Exact verified canonical site address.';

COMMENT ON COLUMN directory.site_ownerships.verified_claim_id IS 'Proof establishing the current address, null for administrative assignment.';

COMMENT ON COLUMN directory.site_ownerships.revision IS 'Monotonic revision within an ownership generation.';

COMMENT ON COLUMN directory.site_ownerships.created_at IS 'Start of this ownership generation.';

COMMENT ON COLUMN directory.site_ownerships.updated_at IS 'Latest verified address change.';

COMMENT ON TRIGGER owner_friend_requests_touch ON directory.owner_friend_link_requests IS 'Maintains request update time.';

COMMENT ON TRIGGER site_audits_apply_owner_effects ON directory.site_audits IS 'Atomically completes owner-related audit effects.';

COMMENT ON TRIGGER site_audits_validate_owner ON directory.site_audits IS 'Requires current ownership for authenticated owner submissions.';

COMMENT ON TRIGGER users_00_anonymize_owner_submissions ON identity.users IS 'Anonymizes authenticated directory submission history before ownership release.';

COMMENT ON TRIGGER users_release_site_ownership ON identity.users IS 'Releases active site ownership when account anonymization completes.';

GRANT ALL ON FUNCTION directory.apply_verified_site_address(p_site uuid, p_user uuid, p_address text, p_ownership uuid) TO api_runtime;

REVOKE ALL ON FUNCTION directory.list_owner_friend_links(p_source_site_id uuid, p_include_inactive boolean) FROM PUBLIC;
GRANT ALL ON FUNCTION directory.list_owner_friend_links(p_source_site_id uuid, p_include_inactive boolean) TO api_runtime;

REVOKE ALL ON FUNCTION directory.remove_owner_friend_link(p_source_site_id uuid, p_target_host text) FROM PUBLIC;
GRANT ALL ON FUNCTION directory.remove_owner_friend_link(p_source_site_id uuid, p_target_host text) TO api_runtime;

GRANT SELECT,INSERT,UPDATE ON TABLE directory.owner_friend_link_requests TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.site_claims TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.site_ownership_events TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.site_ownerships TO api_runtime;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The consolidated baseline is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd

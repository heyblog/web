-- +goose Up
CREATE TABLE directory.site_claims (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 verification application identifier.
    site_id uuid NOT NULL REFERENCES directory.sites(id), -- Registered site whose control is being proven.
    user_id uuid REFERENCES identity.users(id) ON DELETE SET NULL, -- Applicant, anonymized upon account deletion.
    address text NOT NULL, -- Exact normalized address covered by the proof.
    method text NOT NULL CHECK (method IN ('DNS_TXT','META','FILE','MANUAL')), -- DNS_TXT, META, FILE, or MANUAL verification.
    status text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','VERIFIED','REJECTED','CANCELLED')), -- Verification lifecycle status.
    token_hash text, -- SHA256 digest; raw challenge tokens are never stored.
    expires_at timestamptz, -- Automatic challenge deadline; manual requests have none.
    evidence text, -- Applicant public ownership evidence for human review.
    evidence_url text, -- Public evidence URL, displayed without fetching.
    reviewed_by uuid REFERENCES identity.users(id) ON DELETE SET NULL, -- Human reviewer, anonymized upon deletion.
    review_reason text, -- Human decision explanation.
    created_at timestamptz NOT NULL DEFAULT now(), -- Application creation time.
    verified_at timestamptz, -- Successful verification time.
    consumed_at timestamptz, -- Time proof was consumed by an ownership or address change.
 CHECK (address ~ '^https?://'),
 CHECK ((method = 'MANUAL' AND token_hash IS NULL AND expires_at IS NULL AND evidence IS NOT NULL AND btrim(evidence) <> '') OR
        (method <> 'MANUAL' AND token_hash ~ '^[0-9a-f]{64}$' AND expires_at > created_at)),
 CHECK (reviewed_by IS NULL OR reviewed_by <> user_id)
);
CREATE INDEX site_claims_user_idx ON directory.site_claims(user_id,created_at DESC);
CREATE INDEX site_claims_pending_idx ON directory.site_claims(created_at DESC) WHERE status='PENDING';
CREATE TABLE directory.site_ownerships (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- Ownership generation identifier; replacement invalidates pending edits.
    site_id uuid NOT NULL UNIQUE REFERENCES directory.sites(id), -- Uniquely owned registered site.
    user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE, -- Active owner; deletion releases ownership.
    address text NOT NULL CHECK (address ~ '^https?://'), -- Exact verified canonical site address.
    verified_claim_id uuid REFERENCES directory.site_claims(id), -- Proof establishing the current address, null for administrative assignment.
    revision bigint NOT NULL DEFAULT 1, -- Monotonic revision within an ownership generation.
    created_at timestamptz NOT NULL DEFAULT now(), -- Start of this ownership generation.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Latest verified address change.
    CHECK (revision > 0)
);
CREATE INDEX site_ownerships_user_idx ON directory.site_ownerships(user_id);
CREATE TABLE directory.site_ownership_events (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 historical event identifier.
    site_id uuid NOT NULL REFERENCES directory.sites(id), -- Affected registered site.
    user_id uuid REFERENCES identity.users(id) ON DELETE SET NULL, -- Affected owner, anonymized upon deletion.
    actor_id uuid REFERENCES identity.users(id) ON DELETE SET NULL, -- Acting user, anonymized upon deletion.
    action text NOT NULL, -- Ownership event category.
    reason text NOT NULL, -- Decision or operation explanation.
    evidence text, -- Administrative evidence, never fetched automatically.
    created_at timestamptz NOT NULL DEFAULT now(), -- Event creation time.
    CHECK (action IN ('VERIFIED','REVOKED','REASSIGNED','ADDRESS_CHANGED'))
);
CREATE INDEX site_ownership_events_site_idx ON directory.site_ownership_events(site_id,created_at DESC);

-- +goose StatementBegin
CREATE FUNCTION directory.apply_verified_site_address(p_site uuid,p_user uuid,p_address text,p_ownership uuid)
RETURNS void LANGUAGE plpgsql AS $$
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

GRANT SELECT,INSERT,UPDATE,DELETE ON directory.site_claims,directory.site_ownerships,directory.site_ownership_events TO api_runtime;
GRANT EXECUTE ON FUNCTION directory.apply_verified_site_address(uuid,uuid,text,uuid) TO api_runtime;
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
COMMENT ON TABLE directory.site_ownerships IS 'One currently effective owner per registered site.';
COMMENT ON COLUMN directory.site_ownerships.id IS 'Ownership generation identifier; replacement invalidates pending edits.';
COMMENT ON COLUMN directory.site_ownerships.site_id IS 'Uniquely owned registered site.';
COMMENT ON COLUMN directory.site_ownerships.user_id IS 'Active owner; deletion releases ownership.';
COMMENT ON COLUMN directory.site_ownerships.address IS 'Exact verified canonical site address.';
COMMENT ON COLUMN directory.site_ownerships.verified_claim_id IS 'Proof establishing the current address, null for administrative assignment.';
COMMENT ON COLUMN directory.site_ownerships.revision IS 'Monotonic revision within an ownership generation.';
COMMENT ON COLUMN directory.site_ownerships.created_at IS 'Start of this ownership generation.';
COMMENT ON COLUMN directory.site_ownerships.updated_at IS 'Latest verified address change.';
COMMENT ON TABLE directory.site_ownership_events IS 'Historical verification and administrative ownership decisions.';
COMMENT ON COLUMN directory.site_ownership_events.id IS 'UUIDv7 historical event identifier.';
COMMENT ON COLUMN directory.site_ownership_events.site_id IS 'Affected registered site.';
COMMENT ON COLUMN directory.site_ownership_events.user_id IS 'Affected owner, anonymized upon deletion.';
COMMENT ON COLUMN directory.site_ownership_events.actor_id IS 'Acting user, anonymized upon deletion.';
COMMENT ON COLUMN directory.site_ownership_events.action IS 'Ownership event category.';
COMMENT ON COLUMN directory.site_ownership_events.reason IS 'Decision or operation explanation.';
COMMENT ON COLUMN directory.site_ownership_events.evidence IS 'Administrative evidence, never fetched automatically.';
COMMENT ON COLUMN directory.site_ownership_events.created_at IS 'Event creation time.';
COMMENT ON FUNCTION directory.apply_verified_site_address(uuid,uuid,text,uuid) IS 'Consumes exact applicant/site/address proof and atomically changes a matching ownership generation.';
-- +goose StatementBegin
CREATE FUNCTION directory.release_anonymized_site_ownership()
RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER users_release_site_ownership AFTER UPDATE OF deleted_at ON identity.users FOR EACH ROW EXECUTE FUNCTION directory.release_anonymized_site_ownership();
COMMENT ON FUNCTION directory.release_anonymized_site_ownership() IS 'Releases ownership and anonymizes claim history on terminal account deletion.';
COMMENT ON TRIGGER users_release_site_ownership ON identity.users IS 'Releases active site ownership when account anonymization completes.';
-- +goose Down
DROP TRIGGER users_release_site_ownership ON identity.users;
DROP FUNCTION directory.release_anonymized_site_ownership();
DROP FUNCTION directory.apply_verified_site_address(uuid,uuid,text,uuid);
DROP TABLE directory.site_ownership_events;
DROP TABLE directory.site_ownerships;
DROP TABLE directory.site_claims;

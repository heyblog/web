-- +goose Up

ALTER TABLE identity.oauth_identities DROP CONSTRAINT oauth_identities_provider_check;
ALTER TABLE identity.oauth_identities ADD CONSTRAINT oauth_identities_provider_check CHECK (provider IN ('GITHUB', 'BLOGSCLUB'));
COMMENT ON TABLE identity.oauth_identities IS 'External OAuth login identities linked to local accounts.';
COMMENT ON COLUMN identity.oauth_identities.provider IS 'External authentication provider: GITHUB or BLOGSCLUB.'; -- External authentication provider: GITHUB or BLOGSCLUB.

ALTER TABLE directory.site_claims DROP CONSTRAINT site_claims_method_check;
ALTER TABLE directory.site_claims ADD CONSTRAINT site_claims_method_check CHECK (method IN ('DNS_TXT', 'META', 'FILE', 'MANUAL', 'BLOGSCLUB'));
ALTER TABLE directory.site_claims DROP CONSTRAINT site_claims_check;
ALTER TABLE directory.site_claims ADD CONSTRAINT site_claims_check CHECK (
    (method='MANUAL' AND token_hash IS NULL AND expires_at IS NULL AND evidence IS NOT NULL AND btrim(evidence)<>'')
    OR (method='BLOGSCLUB' AND token_hash IS NULL AND expires_at IS NULL)
    OR (method IN ('DNS_TXT','META','FILE') AND token_hash IS NOT NULL AND token_hash ~ '^[0-9a-f]{64}$' AND expires_at IS NOT NULL AND expires_at>created_at)
);
COMMENT ON COLUMN directory.site_claims.method IS 'DNS_TXT, META, FILE, MANUAL, or BLOGSCLUB verification.'; -- DNS_TXT, META, FILE, MANUAL, or BLOGSCLUB verification.
COMMENT ON COLUMN directory.site_claims.expires_at IS 'Challenge deadline; manual and BlogsClub applications have none.'; -- Challenge deadline; manual and BlogsClub applications have none.

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'OAuth provider expansion is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd

-- +goose Up

SELECT directory.backup_check_schema();

CREATE TABLE identity.email_change_requests (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- One-use identifier for the pending change.
    user_id uuid NOT NULL UNIQUE REFERENCES identity.users(id) ON DELETE CASCADE, -- Account requesting the change.
    auth_version integer NOT NULL, -- Authentication generation captured when the request was created.
    old_email text NOT NULL, -- Current verified email captured at request creation.
    new_email text NOT NULL, -- Normalized replacement email.
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(), -- Time the request was created.
    expires_at timestamptz NOT NULL, -- Deadline to complete the request.
    old_verified boolean NOT NULL DEFAULT false, -- Current email control or account reauthentication accepted for this request.
    new_verified boolean NOT NULL DEFAULT false, -- New email control accepted for this request.
    old_code_hash text, -- Keyed digest of the current email verification code.
    new_code_hash text, -- Keyed digest of the new email verification code.
    old_code_expires_at timestamptz, -- Deadline for the current email verification code.
    new_code_expires_at timestamptz, -- Deadline for the new email verification code.
    old_attempts integer NOT NULL DEFAULT 0, -- Failed attempts against the current email verification code.
    new_attempts integer NOT NULL DEFAULT 0, -- Failed attempts against the new email verification code.
    CONSTRAINT email_change_requests_email_check CHECK (
        old_email <> new_email AND old_email <> '' AND new_email <> ''
        AND old_email = lower(btrim(old_email)) AND new_email = lower(btrim(new_email))
    ),
    CONSTRAINT email_change_requests_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT email_change_requests_attempts_check CHECK (
        old_attempts BETWEEN 0 AND 5 AND new_attempts BETWEEN 0 AND 5
    )
);

COMMENT ON TABLE identity.email_change_requests IS 'Transient, account-bound email change and verification state.';
COMMENT ON COLUMN identity.email_change_requests.id IS 'One-use identifier for the pending change.';
COMMENT ON COLUMN identity.email_change_requests.user_id IS 'Account requesting the change.';
COMMENT ON COLUMN identity.email_change_requests.auth_version IS 'Authentication generation captured when the request was created.';
COMMENT ON COLUMN identity.email_change_requests.old_email IS 'Current verified email captured at request creation.';
COMMENT ON COLUMN identity.email_change_requests.new_email IS 'Normalized replacement email.';
COMMENT ON COLUMN identity.email_change_requests.created_at IS 'Time the request was created.';
COMMENT ON COLUMN identity.email_change_requests.expires_at IS 'Deadline to complete the request.';
COMMENT ON COLUMN identity.email_change_requests.old_verified IS 'Current email control or account reauthentication accepted for this request.';
COMMENT ON COLUMN identity.email_change_requests.new_verified IS 'New email control accepted for this request.';
COMMENT ON COLUMN identity.email_change_requests.old_code_hash IS 'Keyed digest of the current email verification code.';
COMMENT ON COLUMN identity.email_change_requests.new_code_hash IS 'Keyed digest of the new email verification code.';
COMMENT ON COLUMN identity.email_change_requests.old_code_expires_at IS 'Deadline for the current email verification code.';
COMMENT ON COLUMN identity.email_change_requests.new_code_expires_at IS 'Deadline for the new email verification code.';
COMMENT ON COLUMN identity.email_change_requests.old_attempts IS 'Failed attempts against the current email verification code.';
COMMENT ON COLUMN identity.email_change_requests.new_attempts IS 'Failed attempts against the new email verification code.';

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE identity.email_change_requests TO api_runtime;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The consolidated baseline is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd

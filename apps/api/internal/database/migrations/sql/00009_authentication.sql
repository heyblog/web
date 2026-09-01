-- +goose Up
CREATE TABLE identity.email_verification_codes (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 email verification code primary key.
    user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE, -- User receiving the verification code.
    email text NOT NULL, -- Normalized email address the code was issued for.
    code_hash text NOT NULL, -- HMAC digest of the six-digit verification code.
    attempt_count integer NOT NULL DEFAULT 0, -- Number of failed verification attempts.
    expires_at timestamptz NOT NULL, -- Time after which the code cannot be accepted.
    consumed_at timestamptz, -- Time the code was successfully consumed.
    created_at timestamptz NOT NULL DEFAULT now(), -- Time the verification code was issued.
    CONSTRAINT email_verification_codes_email_check CHECK (email = lower(btrim(email)) AND email <> ''),
    CONSTRAINT email_verification_codes_hash_check CHECK (btrim(code_hash) <> ''),
    CONSTRAINT email_verification_codes_attempts_check CHECK (attempt_count >= 0 AND attempt_count <= 5),
    CONSTRAINT email_verification_codes_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT email_verification_codes_consumed_check CHECK (consumed_at IS NULL OR consumed_at >= created_at)
);
CREATE INDEX email_verification_codes_user_idx ON identity.email_verification_codes (user_id, created_at DESC);
CREATE INDEX email_verification_codes_expiry_idx ON identity.email_verification_codes (expires_at);

CREATE TABLE identity.password_reset_tokens (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 password reset token primary key.
    user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE, -- User allowed to reset the password.
    email text NOT NULL, -- Normalized email address the token was issued for.
    token_hash text NOT NULL UNIQUE, -- Hash of the one-time password reset token.
    expires_at timestamptz NOT NULL, -- Time after which the token cannot be accepted.
    consumed_at timestamptz, -- Time the token was successfully consumed.
    created_at timestamptz NOT NULL DEFAULT now(), -- Time the password reset token was issued.
    CONSTRAINT password_reset_tokens_email_check CHECK (email = lower(btrim(email)) AND email <> ''),
    CONSTRAINT password_reset_tokens_hash_check CHECK (btrim(token_hash) <> ''),
    CONSTRAINT password_reset_tokens_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT password_reset_tokens_consumed_check CHECK (consumed_at IS NULL OR consumed_at >= created_at)
);
CREATE INDEX password_reset_tokens_user_idx ON identity.password_reset_tokens (user_id, created_at DESC);
CREATE INDEX password_reset_tokens_expiry_idx ON identity.password_reset_tokens (expires_at);

CREATE TABLE identity.user_management_permissions (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 management permission primary key.
    user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE, -- User receiving the permission.
    permission_key text NOT NULL, -- Stable management permission identifier.
    granted_by uuid REFERENCES identity.users(id) ON DELETE SET NULL, -- User that granted the permission.
    created_at timestamptz NOT NULL DEFAULT now(), -- Time the permission was granted.
    CONSTRAINT user_management_permissions_key_check CHECK (permission_key IN ('user.manage', 'site_audit.review', 'feedback.review', 'announcement.manage', 'taxonomy.manage', 'site.manage', 'task.manage', 'log.read')),
    CONSTRAINT user_management_permissions_unique UNIQUE (user_id, permission_key)
);
CREATE INDEX user_management_permissions_user_idx ON identity.user_management_permissions (user_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON identity.email_verification_codes TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON identity.password_reset_tokens TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON identity.user_management_permissions TO api_runtime;

COMMENT ON TABLE identity.email_verification_codes IS 'Short-lived hashed email verification codes for local accounts.';
COMMENT ON TABLE identity.password_reset_tokens IS 'Short-lived one-time password reset tokens.';
COMMENT ON TABLE identity.user_management_permissions IS 'Delegated management permissions assigned to administrator accounts.';
COMMENT ON COLUMN identity.email_verification_codes.id IS 'UUIDv7 email verification code primary key.';
COMMENT ON COLUMN identity.email_verification_codes.user_id IS 'User receiving the verification code.';
COMMENT ON COLUMN identity.email_verification_codes.email IS 'Normalized email address the code was issued for.';
COMMENT ON COLUMN identity.email_verification_codes.code_hash IS 'HMAC digest of the six-digit verification code.';
COMMENT ON COLUMN identity.email_verification_codes.attempt_count IS 'Number of failed verification attempts.';
COMMENT ON COLUMN identity.email_verification_codes.expires_at IS 'Time after which the code cannot be accepted.';
COMMENT ON COLUMN identity.email_verification_codes.consumed_at IS 'Time the code was successfully consumed.';
COMMENT ON COLUMN identity.email_verification_codes.created_at IS 'Time the verification code was issued.';
COMMENT ON COLUMN identity.password_reset_tokens.id IS 'UUIDv7 password reset token primary key.';
COMMENT ON COLUMN identity.password_reset_tokens.user_id IS 'User allowed to reset the password.';
COMMENT ON COLUMN identity.password_reset_tokens.email IS 'Normalized email address the token was issued for.';
COMMENT ON COLUMN identity.password_reset_tokens.token_hash IS 'Hash of the one-time password reset token.';
COMMENT ON COLUMN identity.password_reset_tokens.expires_at IS 'Time after which the token cannot be accepted.';
COMMENT ON COLUMN identity.password_reset_tokens.consumed_at IS 'Time the token was successfully consumed.';
COMMENT ON COLUMN identity.password_reset_tokens.created_at IS 'Time the password reset token was issued.';
COMMENT ON COLUMN identity.user_management_permissions.id IS 'UUIDv7 management permission primary key.';
COMMENT ON COLUMN identity.user_management_permissions.user_id IS 'User receiving the permission.';
COMMENT ON COLUMN identity.user_management_permissions.permission_key IS 'Stable management permission identifier.';
COMMENT ON COLUMN identity.user_management_permissions.granted_by IS 'User that granted the permission.';
COMMENT ON COLUMN identity.user_management_permissions.created_at IS 'Time the permission was granted.';

-- +goose Down
DROP TABLE identity.user_management_permissions;
DROP TABLE identity.password_reset_tokens;
DROP TABLE identity.email_verification_codes;

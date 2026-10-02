-- name: CreateUser :one
INSERT INTO identity.users (
    email,
    username,
    display_name,
    password_hash
) VALUES (
    sqlc.arg(email)::text,
    sqlc.arg(username)::text,
    sqlc.arg(display_name)::text,
    sqlc.narg(password_hash)::text
)
RETURNING id, email, username, display_name, password_hash, role, access_status,
          email_verified_at, auth_version, profile, settings, last_login_at,
          deletion_requested_at, deletion_scheduled_for, deleted_at, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, email, username, display_name, password_hash, role, access_status,
       email_verified_at, auth_version, profile, settings, last_login_at,
       deletion_requested_at, deletion_scheduled_for, deleted_at, created_at, updated_at
  FROM identity.users
 WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, username, display_name, password_hash, role, access_status,
       email_verified_at, auth_version, profile, settings, last_login_at,
       deletion_requested_at, deletion_scheduled_for, deleted_at, created_at, updated_at
  FROM identity.users
 WHERE email = sqlc.arg(email)::text;

-- name: GetUserByUsername :one
SELECT id, email, username, display_name, password_hash, role, access_status,
       email_verified_at, auth_version, profile, settings, last_login_at,
       deletion_requested_at, deletion_scheduled_for, deleted_at, created_at, updated_at
  FROM identity.users
 WHERE username = $1;

-- name: UpdateUserProfile :one
UPDATE identity.users
   SET display_name = $2,
       profile = $3,
       settings = $4
 WHERE id = $1
   AND access_status = 'ACTIVE'
   AND deletion_requested_at IS NULL
   AND deleted_at IS NULL
RETURNING id, email, username, display_name, password_hash, role, access_status,
          email_verified_at, auth_version, profile, settings, last_login_at,
          deletion_requested_at, deletion_scheduled_for, deleted_at, created_at, updated_at;

-- name: RecordUserLogin :one
UPDATE identity.users
   SET last_login_at = clock_timestamp()
 WHERE id = $1
   AND access_status = 'ACTIVE'
   AND deletion_requested_at IS NULL
   AND deleted_at IS NULL
RETURNING id, email, username, display_name, password_hash, role, access_status,
          email_verified_at, auth_version, profile, settings, last_login_at,
          deletion_requested_at, deletion_scheduled_for, deleted_at, created_at, updated_at;

-- name: SuspendUser :one
UPDATE identity.users
   SET access_status = 'SUSPENDED'
 WHERE id = $1
   AND access_status = 'ACTIVE'
   AND deletion_requested_at IS NULL
   AND deleted_at IS NULL
RETURNING id, email, username, display_name, password_hash, role, access_status,
          email_verified_at, auth_version, profile, settings, last_login_at,
          deletion_requested_at, deletion_scheduled_for, deleted_at, created_at, updated_at;

-- name: ActivateUser :one
UPDATE identity.users
   SET access_status = 'ACTIVE'
 WHERE id = $1
   AND access_status = 'SUSPENDED'
   AND deletion_requested_at IS NULL
   AND deleted_at IS NULL
RETURNING id, email, username, display_name, password_hash, role, access_status,
          email_verified_at, auth_version, profile, settings, last_login_at,
          deletion_requested_at, deletion_scheduled_for, deleted_at, created_at, updated_at;

-- name: RequestUserDeletion :one
WITH request_time AS (
    SELECT clock_timestamp() AS requested_at
)
UPDATE identity.users
   SET access_status = 'SUSPENDED',
       deletion_requested_at = request_time.requested_at,
       deletion_scheduled_for = request_time.requested_at + interval '30 days'
  FROM request_time
 WHERE id = $1
   AND access_status = 'ACTIVE'
   AND deletion_requested_at IS NULL
   AND deleted_at IS NULL
RETURNING identity.users.id, identity.users.email, identity.users.username,
          identity.users.display_name, identity.users.password_hash, identity.users.role,
          identity.users.access_status, identity.users.email_verified_at,
          identity.users.auth_version, identity.users.profile, identity.users.settings,
          identity.users.last_login_at, identity.users.deletion_requested_at,
          identity.users.deletion_scheduled_for, identity.users.deleted_at,
          identity.users.created_at, identity.users.updated_at;

-- name: CancelUserDeletion :one
UPDATE identity.users
   SET access_status = 'ACTIVE',
       deletion_requested_at = NULL,
       deletion_scheduled_for = NULL
 WHERE id = $1
   AND deletion_requested_at IS NOT NULL
   AND deletion_scheduled_for > clock_timestamp()
   AND deleted_at IS NULL
RETURNING id, email, username, display_name, password_hash, role, access_status,
          email_verified_at, auth_version, profile, settings, last_login_at,
          deletion_requested_at, deletion_scheduled_for, deleted_at, created_at, updated_at;

-- name: CompleteUserDeletion :one
UPDATE identity.users
   SET deleted_at = clock_timestamp()
 WHERE id = $1
   AND deletion_requested_at IS NOT NULL
   AND deletion_scheduled_for <= clock_timestamp()
   AND deleted_at IS NULL
RETURNING id, email, username, display_name, password_hash, role, access_status,
          email_verified_at, auth_version, profile, settings, last_login_at,
          deletion_requested_at, deletion_scheduled_for, deleted_at, created_at, updated_at;

-- name: UpsertGitHubIdentity :one
INSERT INTO identity.oauth_identities (
    user_id,
    provider,
    provider_user_id,
    provider_login,
    profile
) SELECT user_account.id, 'GITHUB', sqlc.arg(provider_user_id)::text,
         sqlc.narg(provider_login)::text, sqlc.arg(profile)::jsonb
    FROM identity.users AS user_account
   WHERE user_account.id = sqlc.arg(user_id)::uuid
     AND user_account.access_status = 'ACTIVE'
     AND user_account.deletion_requested_at IS NULL
     AND user_account.deleted_at IS NULL
ON CONFLICT (provider, provider_user_id) DO UPDATE
   SET provider_login = EXCLUDED.provider_login,
       profile = EXCLUDED.profile
 WHERE identity.oauth_identities.user_id = EXCLUDED.user_id
RETURNING id, user_id, provider, provider_user_id, provider_login, profile, created_at, updated_at;

-- name: GetGitHubIdentity :one
SELECT id, user_id, provider, provider_user_id, provider_login, profile, created_at, updated_at
  FROM identity.oauth_identities
 WHERE provider = 'GITHUB' AND provider_user_id = $1;

-- name: ListUserOAuthIdentities :many
SELECT id, user_id, provider, provider_user_id, provider_login, profile, created_at, updated_at
  FROM identity.oauth_identities
 WHERE user_id = $1
 ORDER BY provider;

-- name: UnlinkOAuthIdentity :exec
WITH removed_identity AS (
    DELETE FROM identity.oauth_identities AS oauth_identity
     USING identity.users AS user_account
     WHERE oauth_identity.user_id = $1
       AND oauth_identity.provider = $2
       AND user_account.id = oauth_identity.user_id
       AND user_account.access_status = 'ACTIVE'
       AND user_account.deletion_requested_at IS NULL
       AND user_account.deleted_at IS NULL
     RETURNING oauth_identity.user_id
)
UPDATE identity.users
   SET auth_version = auth_version + 1
 WHERE id IN (SELECT user_id FROM removed_identity)
   AND deletion_requested_at IS NULL
   AND deleted_at IS NULL;

-- name: SetUserEmailVerified :exec
UPDATE identity.users
   SET email_verified_at = clock_timestamp()
 WHERE id = sqlc.arg(id)::uuid
   AND deleted_at IS NULL;

-- name: SetUserPassword :exec
UPDATE identity.users
   SET password_hash = sqlc.arg(password_hash)::text,
       auth_version = auth_version + 1
 WHERE id = sqlc.arg(id)::uuid
   AND access_status = 'ACTIVE'
   AND deleted_at IS NULL;

-- name: DeleteEmailVerificationCodes :exec
DELETE FROM identity.email_verification_codes
 WHERE user_id = sqlc.arg(user_id)::uuid;

-- name: CreateEmailVerificationCode :exec
INSERT INTO identity.email_verification_codes (user_id, email, code_hash, expires_at)
VALUES (sqlc.arg(user_id)::uuid, sqlc.arg(email)::text, sqlc.arg(code_hash)::text, sqlc.arg(expires_at)::timestamptz);

-- name: GetLatestEmailVerificationCode :one
SELECT id, user_id, email, code_hash, attempt_count, expires_at, consumed_at, created_at
  FROM identity.email_verification_codes
 WHERE email = sqlc.arg(email)::text AND consumed_at IS NULL
 ORDER BY created_at DESC
 LIMIT 1
 FOR UPDATE;

-- name: IncrementEmailVerificationAttempts :exec
UPDATE identity.email_verification_codes
   SET attempt_count = attempt_count + 1
 WHERE id = sqlc.arg(id)::uuid;

-- name: ConsumeEmailVerificationCode :exec
UPDATE identity.email_verification_codes
   SET consumed_at = clock_timestamp()
 WHERE id = sqlc.arg(id)::uuid AND consumed_at IS NULL;

-- name: DeletePasswordResetTokens :exec
DELETE FROM identity.password_reset_tokens
 WHERE user_id = sqlc.arg(user_id)::uuid;

-- name: CreatePasswordResetToken :exec
INSERT INTO identity.password_reset_tokens (user_id, email, token_hash, expires_at)
VALUES (sqlc.arg(user_id)::uuid, sqlc.arg(email)::text, sqlc.arg(token_hash)::text, sqlc.arg(expires_at)::timestamptz);

-- name: GetPasswordResetToken :one
SELECT id, user_id, email, token_hash, expires_at, consumed_at, created_at
  FROM identity.password_reset_tokens
 WHERE token_hash = sqlc.arg(token_hash)::text AND consumed_at IS NULL
 FOR UPDATE;

-- name: ConsumePasswordResetToken :exec
UPDATE identity.password_reset_tokens
   SET consumed_at = clock_timestamp()
 WHERE id = sqlc.arg(id)::uuid AND consumed_at IS NULL;

-- name: ListUserManagementPermissions :many
SELECT permission_key
  FROM identity.user_management_permissions
 WHERE user_id = sqlc.arg(user_id)::uuid
 ORDER BY permission_key;

-- name: ListUsersForManagement :many
SELECT id, email, username, display_name, password_hash, role, access_status,
       email_verified_at, auth_version, profile, settings, last_login_at,
       deletion_requested_at, deletion_scheduled_for, deleted_at, created_at, updated_at
  FROM identity.users
 WHERE deleted_at IS NULL
 ORDER BY created_at DESC;

-- name: SetUserRole :exec
UPDATE identity.users
   SET role = sqlc.arg(role)::text
 WHERE id = sqlc.arg(id)::uuid AND deleted_at IS NULL;

-- name: BumpUserAuthVersion :exec
UPDATE identity.users
   SET auth_version = auth_version + 1
 WHERE id = sqlc.arg(id)::uuid AND deleted_at IS NULL;

-- name: DeleteUserManagementPermissions :exec
DELETE FROM identity.user_management_permissions
 WHERE user_id = sqlc.arg(user_id)::uuid;

-- name: CreateUserManagementPermission :exec
INSERT INTO identity.user_management_permissions (user_id, permission_key, granted_by)
VALUES (sqlc.arg(user_id)::uuid, sqlc.arg(permission_key)::text, sqlc.arg(granted_by)::uuid);

-- name: GetUserGitHubIdentity :one
SELECT id, user_id, provider, provider_user_id, provider_login, profile, created_at, updated_at
  FROM identity.oauth_identities
 WHERE provider = 'GITHUB' AND user_id = sqlc.arg(user_id)::uuid;

-- name: DeleteUserGitHubIdentity :exec
DELETE FROM identity.oauth_identities
 WHERE provider = 'GITHUB' AND user_id = sqlc.arg(user_id)::uuid;

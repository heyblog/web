-- name: CreateUser :one
INSERT INTO identity.users (
    email,
    username,
    display_name,
    password_hash
) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM identity.users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM identity.users WHERE email = $1;

-- name: GetUserByUsername :one
SELECT * FROM identity.users WHERE username = $1;

-- name: UpdateUserProfile :one
UPDATE identity.users
   SET display_name = $2,
       profile = $3,
       settings = $4
 WHERE id = $1
RETURNING *;

-- name: RecordUserLogin :one
UPDATE identity.users
   SET last_login_at = clock_timestamp()
 WHERE id = $1
RETURNING *;

-- name: UpsertGitHubIdentity :one
INSERT INTO identity.oauth_identities (
    user_id,
    provider,
    provider_user_id,
    provider_login,
    profile
) VALUES ($1, 'GITHUB', $2, $3, $4)
ON CONFLICT (provider, provider_user_id) DO UPDATE
   SET provider_login = EXCLUDED.provider_login,
       profile = EXCLUDED.profile
 WHERE identity.oauth_identities.user_id = EXCLUDED.user_id
RETURNING *;

-- name: GetGitHubIdentity :one
SELECT *
  FROM identity.oauth_identities
 WHERE provider = 'GITHUB' AND provider_user_id = $1;

-- name: ListUserOAuthIdentities :many
SELECT *
  FROM identity.oauth_identities
 WHERE user_id = $1
 ORDER BY provider;

-- name: UnlinkOAuthIdentity :exec
DELETE FROM identity.oauth_identities
 WHERE user_id = $1 AND provider = $2;

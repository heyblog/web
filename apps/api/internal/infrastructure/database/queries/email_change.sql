-- name: LockEmailChangeRequest :one
SELECT * FROM identity.email_change_requests WHERE user_id = sqlc.arg(user_id)::uuid FOR UPDATE;

-- name: CreateEmailChangeRequest :one
INSERT INTO identity.email_change_requests (user_id, auth_version, old_email, new_email, expires_at)
VALUES (sqlc.arg(user_id)::uuid, sqlc.arg(auth_version)::integer, sqlc.arg(old_email)::text,
        sqlc.arg(new_email)::text, clock_timestamp() + interval '7 days')
RETURNING *;

-- name: DeleteEmailChangeRequest :exec
DELETE FROM identity.email_change_requests WHERE id = sqlc.arg(id)::uuid;

-- name: IssueEmailChangeCode :exec
UPDATE identity.email_change_requests
SET old_code_hash = CASE WHEN sqlc.arg(is_old)::boolean THEN sqlc.arg(code_hash)::text ELSE old_code_hash END,
    new_code_hash = CASE WHEN sqlc.arg(is_old)::boolean THEN new_code_hash ELSE sqlc.arg(code_hash)::text END,
    old_code_expires_at = CASE WHEN sqlc.arg(is_old)::boolean THEN sqlc.arg(code_expiry)::timestamptz ELSE old_code_expires_at END,
    new_code_expires_at = CASE WHEN sqlc.arg(is_old)::boolean THEN new_code_expires_at ELSE sqlc.arg(code_expiry)::timestamptz END,
    old_attempts = CASE WHEN sqlc.arg(is_old)::boolean THEN 0 ELSE old_attempts END,
    new_attempts = CASE WHEN sqlc.arg(is_old)::boolean THEN new_attempts ELSE 0 END
WHERE id = sqlc.arg(id)::uuid;

-- name: IncrementEmailChangeCodeAttempts :exec
UPDATE identity.email_change_requests
SET old_attempts = old_attempts + CASE WHEN sqlc.arg(is_old)::boolean THEN 1 ELSE 0 END,
    new_attempts = new_attempts + CASE WHEN sqlc.arg(is_old)::boolean THEN 0 ELSE 1 END
WHERE id = sqlc.arg(id)::uuid;

-- name: MarkEmailChangeVerified :one
UPDATE identity.email_change_requests
SET old_verified = old_verified OR sqlc.arg(is_old)::boolean,
    new_verified = new_verified OR NOT sqlc.arg(is_old)::boolean,
    old_code_hash = CASE WHEN sqlc.arg(is_old)::boolean THEN NULL ELSE old_code_hash END,
    new_code_hash = CASE WHEN sqlc.arg(is_old)::boolean THEN new_code_hash ELSE NULL END,
    old_code_expires_at = CASE WHEN sqlc.arg(is_old)::boolean THEN NULL ELSE old_code_expires_at END,
    new_code_expires_at = CASE WHEN sqlc.arg(is_old)::boolean THEN new_code_expires_at ELSE NULL END
WHERE id = sqlc.arg(id)::uuid
RETURNING *;

-- name: ChangeUserEmail :execrows
UPDATE identity.users AS account
SET email = request.new_email
FROM identity.email_change_requests AS request
WHERE account.id = request.user_id
  AND request.id = sqlc.arg(request_id)::uuid
  AND request.expires_at > clock_timestamp();

-- name: RecordPasswordLogin :one
UPDATE identity.users
SET last_login_at = clock_timestamp()
WHERE id = sqlc.arg(id)::uuid
  AND auth_version = sqlc.arg(auth_version)::integer
  AND access_status = 'ACTIVE'
  AND deletion_requested_at IS NULL
  AND deleted_at IS NULL
RETURNING *;

-- name: SetSessionUserPassword :one
UPDATE identity.users
SET password_hash = sqlc.arg(password_hash)::text,
    settings = CASE
        WHEN password_hash IS NULL THEN jsonb_set(settings, '{email_change_unproven_password}', 'true'::jsonb)
        ELSE settings
    END,
    auth_version = auth_version + 1
WHERE id = sqlc.arg(id)::uuid
  AND auth_version = sqlc.arg(auth_version)::integer
  AND access_status = 'ACTIVE'
  AND deletion_requested_at IS NULL
  AND deleted_at IS NULL
RETURNING *;

-- name: HasUnprovenEmailChangePassword :one
SELECT (settings @> '{"email_change_unproven_password":true}'::jsonb)::boolean AS unproven
FROM identity.users
WHERE id = sqlc.arg(id)::uuid;

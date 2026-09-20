-- name: CreateAPIClient :one
INSERT INTO identity.api_clients (name, description, audience, created_by)
VALUES (
    sqlc.arg(name)::text,
    sqlc.arg(description)::text,
    sqlc.arg(audience)::text,
    sqlc.arg(created_by)::uuid
)
RETURNING id, name, description, audience, disabled_at, created_by, created_at, updated_at;

-- name: CreateAPIClientScope :exec
INSERT INTO identity.api_client_scopes (client_id, scope)
VALUES (sqlc.arg(client_id)::uuid, sqlc.arg(scope)::text);

-- name: DeleteAPIClientScopes :exec
DELETE FROM identity.api_client_scopes
 WHERE client_id = sqlc.arg(client_id)::uuid;

-- name: CreateAPIKey :one
INSERT INTO identity.api_keys (
    client_id,
    public_id,
    key_prefix,
    secret_hash,
    expires_at,
    rotated_from_id,
    created_by
) VALUES (
    sqlc.arg(client_id)::uuid,
    sqlc.arg(public_id)::text,
    sqlc.arg(key_prefix)::text,
    sqlc.arg(secret_hash)::bytea,
    sqlc.narg(expires_at)::timestamptz,
    sqlc.narg(rotated_from_id)::uuid,
    sqlc.arg(created_by)::uuid
)
RETURNING id, client_id, public_id, key_prefix, secret_hash, expires_at, last_used_at,
          rotated_from_id, created_by, revoked_at, revoked_by, created_at;

-- name: ListAPIClients :many
SELECT id, name, description, audience, disabled_at, created_by, created_at, updated_at
  FROM identity.api_clients
 ORDER BY created_at DESC, id DESC;

-- name: ListAPIClientScopes :many
SELECT client_id, scope, created_at
  FROM identity.api_client_scopes
 ORDER BY client_id, scope;

-- name: ListAPIClientScopesByClient :many
SELECT scope
  FROM identity.api_client_scopes
 WHERE client_id = sqlc.arg(client_id)::uuid
 ORDER BY scope;

-- name: ListAPIKeys :many
SELECT id, client_id, public_id, key_prefix, expires_at, last_used_at, rotated_from_id,
       created_at, revoked_at
  FROM identity.api_keys
 ORDER BY client_id, created_at DESC, id DESC;

-- name: GetAPIClient :one
SELECT id, name, description, audience, disabled_at, created_by, created_at, updated_at
  FROM identity.api_clients
 WHERE id = sqlc.arg(id)::uuid
 FOR UPDATE;

-- name: GetLatestActiveAPIKey :one
SELECT id, client_id, public_id, key_prefix, secret_hash, expires_at, last_used_at,
       rotated_from_id, created_by, revoked_at, revoked_by, created_at
  FROM identity.api_keys
 WHERE client_id = sqlc.arg(client_id)::uuid
   AND revoked_at IS NULL
   AND (expires_at IS NULL OR expires_at > sqlc.arg(now)::timestamptz)
 ORDER BY created_at DESC, id DESC
 LIMIT 1
 FOR UPDATE;

-- name: UpdateAPIClient :one
UPDATE identity.api_clients
   SET name = sqlc.arg(name)::text,
       description = sqlc.arg(description)::text,
       disabled_at = CASE
           WHEN sqlc.arg(enabled)::boolean THEN NULL
           ELSE coalesce(disabled_at, clock_timestamp())
       END
 WHERE id = sqlc.arg(id)::uuid
RETURNING id, name, description, audience, disabled_at, created_by, created_at, updated_at;

-- name: RetireAPIKeyForRotation :exec
UPDATE identity.api_keys
   SET expires_at = CASE
           WHEN sqlc.arg(overlap_until)::timestamptz <= sqlc.arg(now)::timestamptz THEN expires_at
           ELSE least(expires_at, sqlc.arg(overlap_until)::timestamptz)
       END,
       revoked_at = CASE
           WHEN sqlc.arg(overlap_until)::timestamptz <= sqlc.arg(now)::timestamptz THEN sqlc.arg(now)::timestamptz
           ELSE revoked_at
       END,
       revoked_by = CASE
           WHEN sqlc.arg(overlap_until)::timestamptz <= sqlc.arg(now)::timestamptz THEN sqlc.arg(actor_id)::uuid
           ELSE revoked_by
       END
 WHERE id = sqlc.arg(id)::uuid
   AND revoked_at IS NULL;

-- name: RevokeAPIKey :execrows
UPDATE identity.api_keys
   SET revoked_at = sqlc.arg(now)::timestamptz,
       revoked_by = sqlc.arg(actor_id)::uuid
 WHERE id = sqlc.arg(id)::uuid
   AND revoked_at IS NULL;

-- name: FindAPIKeyCredential :one
SELECT api_key.id AS key_id,
       api_key.secret_hash,
       api_key.expires_at,
       api_key.revoked_at,
       api_key.last_used_at,
       client.id AS client_id,
       client.audience,
       client.disabled_at AS client_disabled_at,
       ARRAY(
           SELECT client_scope.scope
             FROM identity.api_client_scopes AS client_scope
            WHERE client_scope.client_id = client.id
            ORDER BY client_scope.scope
       )::text[] AS scopes
  FROM identity.api_keys AS api_key
  JOIN identity.api_clients AS client ON client.id = api_key.client_id
 WHERE api_key.public_id = sqlc.arg(public_id)::text;

-- name: TouchAPIKeyUsage :exec
UPDATE identity.api_keys
   SET last_used_at = sqlc.arg(used_at)::timestamptz
 WHERE id = sqlc.arg(id)::uuid
   AND (last_used_at IS NULL OR last_used_at < sqlc.arg(used_at)::timestamptz - interval '15 minutes');

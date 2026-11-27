-- name: ListEmailChangeSites :many
SELECT s.short_id, s.name, (s.scheme || '://' || s.normalized_host || s.base_path)::text AS address,
       coalesce(proof.status, 'required')::text AS status, coalesce(proof.id, '')::text AS claim_id
FROM directory.site_ownerships o JOIN directory.sites s ON s.id = o.site_id
LEFT JOIN LATERAL (
    SELECT lower(c.status)::text AS status, c.id::text
    FROM directory.site_claims c
    WHERE c.email_change_id = sqlc.narg(request_id)::uuid AND c.email_change_ownership_id = o.id
      AND c.user_id = o.user_id AND c.site_id = s.id AND c.status IN ('VERIFIED', 'PENDING', 'REJECTED')
      AND c.address = s.scheme || '://' || s.normalized_host || s.base_path
    ORDER BY (c.status = 'VERIFIED') DESC, c.created_at DESC, c.id DESC LIMIT 1
) proof ON true
WHERE o.user_id = sqlc.arg(user_id)::uuid AND s.visibility <> 'REMOVED' AND s.blog_status = 'BLOG'
ORDER BY s.name, s.short_id;

-- name: CompleteEmailChangeSiteProofs :one
SELECT directory.complete_email_change_site_proofs(sqlc.arg(user_id)::uuid, sqlc.arg(request_id)::uuid)::boolean;

-- name: CloseEmailChangeSiteClaims :exec
UPDATE directory.site_claims
SET status = 'CANCELLED', reviewed_at = clock_timestamp()
WHERE email_change_id = sqlc.arg(request_id)::uuid AND status = 'PENDING';

-- name: CloseInactiveEmailChangeSiteClaims :exec
WITH inactive AS (
    SELECT c.id FROM directory.site_claims c
    WHERE c.email_change_id IS NOT NULL AND c.status = 'PENDING'
      AND NOT EXISTS (
          SELECT 1 FROM identity.email_change_requests r JOIN identity.users u ON u.id = r.user_id
          WHERE r.id = c.email_change_id AND r.user_id = c.user_id AND r.auth_version = u.auth_version
            AND u.access_status = 'ACTIVE' AND u.deleted_at IS NULL AND u.deletion_requested_at IS NULL
            AND r.expires_at > clock_timestamp()
      )
    LIMIT 100 FOR UPDATE OF c SKIP LOCKED
)
UPDATE directory.site_claims c SET status = 'CANCELLED', reviewed_at = clock_timestamp()
FROM inactive WHERE c.id = inactive.id;

-- name: GetSiteEmailChangeRequest :one
SELECT r.expires_at, (r.expires_at > clock_timestamp() AND r.auth_version = u.auth_version)::boolean AS active
FROM identity.email_change_requests r JOIN identity.users u ON u.id = r.user_id
WHERE r.id = sqlc.arg(request_id)::uuid AND r.user_id = sqlc.arg(user_id)::uuid
  AND u.access_status = 'ACTIVE' AND u.deleted_at IS NULL AND u.deletion_requested_at IS NULL;

-- name: FindPendingEmailChangeSiteClaim :one
SELECT id::text FROM directory.site_claims
WHERE email_change_id = sqlc.arg(request_id)::uuid AND email_change_ownership_id = sqlc.arg(ownership_id)::uuid
  AND address = sqlc.arg(address)::text AND method = sqlc.arg(method)::text AND status = 'PENDING'
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: ClaimSite :one
SELECT id::text, short_id,(scheme || '://' || normalized_host || base_path)::text AS address,visibility FROM directory.sites WHERE short_id=$1 FOR UPDATE;
-- name: CreateSiteClaim :one
INSERT INTO directory.site_claims(site_id,user_id,address,method,token_hash,expires_at,evidence,evidence_url)
VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8) RETURNING id::text;
-- name: GetSiteClaim :one
SELECT c.id::text,c.site_id::text,coalesce(c.user_id::text,'')::text AS user_id,s.short_id,c.address,c.method,c.status,c.token_hash,c.expires_at,c.evidence,c.evidence_url,c.review_reason,c.created_at,c.verified_at,c.consumed_at FROM directory.site_claims c JOIN directory.sites s ON s.id=c.site_id WHERE c.id=$1::uuid;
-- name: LockSiteClaim :one
SELECT c.id::text,c.site_id::text,coalesce(c.user_id::text,'')::text AS user_id,s.short_id,c.address,c.method,c.status,c.token_hash,c.expires_at,c.evidence,c.evidence_url,c.review_reason,c.created_at,c.verified_at,c.consumed_at FROM directory.site_claims c JOIN directory.sites s ON s.id=c.site_id WHERE c.id=$1::uuid FOR UPDATE OF c;
-- name: RefreshSiteOwnership :exec
UPDATE directory.site_ownerships SET address=$2,verified_claim_id=$3::uuid,revision=revision+1,updated_at=now() WHERE site_id=$1::uuid;
-- name: ListSiteClaims :many
SELECT c.id::text FROM directory.site_claims c WHERE sqlc.narg(user_id)::uuid IS NULL OR c.user_id=sqlc.narg(user_id)::uuid ORDER BY c.created_at DESC LIMIT $1 OFFSET $2;
-- name: GetSiteOwnership :one
SELECT o.id::text,o.site_id::text,o.user_id::text,o.address,o.revision FROM directory.site_ownerships o WHERE o.site_id=$1::uuid FOR UPDATE;
-- name: CreateSiteOwnership :exec
INSERT INTO directory.site_ownerships(site_id,user_id,address,verified_claim_id) VALUES($1::uuid,$2::uuid,$3,$4::uuid);
-- name: DeleteSiteOwnership :exec
DELETE FROM directory.site_ownerships WHERE site_id=$1::uuid;
-- name: ResolveOwnershipUser :one
SELECT id::text FROM identity.users WHERE id=$1::uuid AND access_status='ACTIVE' AND deleted_at IS NULL;
-- name: FinishSiteClaim :execrows
UPDATE directory.site_claims SET status=$2,reviewed_by=$3::uuid,review_reason=$4,verified_at=CASE WHEN $2='VERIFIED' THEN clock_timestamp() ELSE NULL END,consumed_at=CASE WHEN $5::boolean THEN clock_timestamp() ELSE NULL END
WHERE id=$1::uuid AND status='PENDING'
AND ($2 <> 'VERIFIED' OR method='MANUAL' OR expires_at > clock_timestamp());
-- name: SiteOwnershipEvent :exec
INSERT INTO directory.site_ownership_events(site_id,user_id,actor_id,action,reason,evidence) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6);

-- name: ClaimSite :one
SELECT id::text, short_id,name,(scheme || '://' || normalized_host || base_path)::text AS address,visibility FROM directory.sites WHERE short_id=$1;
-- name: LockClaimSite :one
SELECT id::text, short_id,name,(scheme || '://' || normalized_host || base_path)::text AS address,visibility FROM directory.sites WHERE short_id=$1 FOR UPDATE;
-- name: CreateSiteClaim :one
INSERT INTO directory.site_claims(site_id,user_id,address,method,token_hash,expires_at,evidence,evidence_url)
VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8) RETURNING id::text;
-- name: GetSiteClaim :one
SELECT c.id::text,c.site_id::text,coalesce(c.user_id::text,'')::text AS user_id,s.short_id,s.name AS site_name,(s.scheme || '://' || s.normalized_host || s.base_path)::text AS current_address,s.visibility,c.address,c.method,c.status,c.token_hash,c.expires_at,c.evidence,c.evidence_url,c.review_reason,c.created_at,c.verified_at,c.consumed_at,coalesce(o.user_id::text,'')::text AS owner_user_id,coalesce(o.address,'')::text AS owner_address,coalesce(u.username,'')::text AS applicant_username,coalesce(u.display_name,'')::text AS applicant_display_name FROM directory.site_claims c JOIN directory.sites s ON s.id=c.site_id LEFT JOIN directory.site_ownerships o ON o.site_id=c.site_id LEFT JOIN identity.users u ON u.id=c.user_id WHERE c.id=$1::uuid;
-- name: LockSiteClaim :one
SELECT c.id::text,c.site_id::text,coalesce(c.user_id::text,'')::text AS user_id,s.short_id,s.name AS site_name,(s.scheme || '://' || s.normalized_host || s.base_path)::text AS current_address,s.visibility,c.address,c.method,c.status,c.token_hash,c.expires_at,c.evidence,c.evidence_url,c.review_reason,c.created_at,c.verified_at,c.consumed_at,coalesce(o.user_id::text,'')::text AS owner_user_id,coalesce(o.address,'')::text AS owner_address,coalesce(u.username,'')::text AS applicant_username,coalesce(u.display_name,'')::text AS applicant_display_name FROM directory.site_claims c JOIN directory.sites s ON s.id=c.site_id LEFT JOIN directory.site_ownerships o ON o.site_id=c.site_id LEFT JOIN identity.users u ON u.id=c.user_id WHERE c.id=$1::uuid FOR UPDATE OF c;
-- name: RefreshSiteOwnership :exec
UPDATE directory.site_ownerships SET address=$2,verified_claim_id=$3::uuid,revision=revision+1,updated_at=now() WHERE site_id=$1::uuid;
-- name: ListSiteClaims :many
SELECT c.id::text,c.site_id::text,coalesce(c.user_id::text,'')::text AS user_id,s.short_id,s.name AS site_name,(s.scheme || '://' || s.normalized_host || s.base_path)::text AS current_address,s.visibility,c.address,c.method,c.status,c.token_hash,c.expires_at,c.evidence,c.evidence_url,c.review_reason,c.created_at,c.verified_at,c.consumed_at,coalesce(o.user_id::text,'')::text AS owner_user_id,coalesce(o.address,'')::text AS owner_address,coalesce(u.username,'')::text AS applicant_username,coalesce(u.display_name,'')::text AS applicant_display_name FROM directory.site_claims c JOIN directory.sites s ON s.id=c.site_id LEFT JOIN directory.site_ownerships o ON o.site_id=c.site_id LEFT JOIN identity.users u ON u.id=c.user_id
WHERE (sqlc.narg(user_id)::uuid IS NULL OR c.user_id=sqlc.narg(user_id)::uuid)
AND (sqlc.narg(site_id)::uuid IS NULL OR c.site_id=sqlc.narg(site_id)::uuid)
AND (sqlc.arg(status)::text='' OR c.status=sqlc.arg(status)::text)
AND (sqlc.arg(method)::text='' OR c.method=sqlc.arg(method)::text)
ORDER BY c.created_at DESC,c.id DESC LIMIT NULLIF(sqlc.arg(page_limit)::integer,0) OFFSET sqlc.arg(page_offset);
-- name: GetSiteOwnership :one
SELECT o.id::text,o.site_id::text,o.user_id::text,o.address,o.revision,u.username,u.display_name FROM directory.site_ownerships o JOIN identity.users u ON u.id=o.user_id WHERE o.site_id=$1::uuid;
-- name: CreateSiteOwnership :exec
INSERT INTO directory.site_ownerships(site_id,user_id,address,verified_claim_id) VALUES($1::uuid,$2::uuid,$3,$4::uuid);
-- name: DeleteSiteOwnership :exec
DELETE FROM directory.site_ownerships WHERE site_id=$1::uuid;
-- name: ResolveOwnershipUser :one
SELECT id::text FROM identity.users WHERE id=$1::uuid AND access_status='ACTIVE' AND deleted_at IS NULL;
-- name: FinishSiteClaim :execrows
UPDATE directory.site_claims SET status=$2,reviewed_by=$3::uuid,review_reason=$4,verified_at=CASE WHEN $2='VERIFIED' THEN clock_timestamp() ELSE NULL END,consumed_at=CASE WHEN $5::boolean THEN clock_timestamp() ELSE NULL END
WHERE id=$1::uuid AND status='PENDING'
AND ($2 <> 'VERIFIED' OR method IN ('MANUAL','BLOGSCLUB') OR expires_at > clock_timestamp());
-- name: SiteOwnershipEvent :exec
INSERT INTO directory.site_ownership_events(site_id,user_id,actor_id,action,reason,evidence) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6);

-- name: LockSiteOwnership :one
SELECT o.id::text,o.site_id::text,o.user_id::text,o.address,o.revision FROM directory.site_ownerships o WHERE o.site_id=$1::uuid FOR UPDATE;
-- name: FindPendingSiteClaim :one
SELECT id::text FROM directory.site_claims WHERE site_id=$1::uuid AND user_id=$2::uuid AND address=$3 AND method=$4 AND status='PENDING' ORDER BY created_at DESC,id DESC LIMIT 1;
-- name: RegenerateSiteClaim :exec
UPDATE directory.site_claims SET token_hash=$2,expires_at=$3 WHERE id=$1::uuid AND status='PENDING' AND method IN ('DNS_TXT','META','FILE');
-- name: InvalidateSiteClaims :exec
UPDATE directory.site_claims SET status='CANCELLED' WHERE site_id=$1::uuid AND (status='PENDING' OR (status='VERIFIED' AND consumed_at IS NULL));
-- name: ListSiteOwnershipEvents :many
SELECT id::text,coalesce(user_id::text,'')::text AS user_id,coalesce(actor_id::text,'')::text AS actor_id,action,reason,evidence,created_at FROM directory.site_ownership_events WHERE site_id=$1::uuid ORDER BY created_at DESC,id DESC;

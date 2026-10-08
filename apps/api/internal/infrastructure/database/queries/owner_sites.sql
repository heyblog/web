-- name: ListAccountSites :many
SELECT s.* FROM directory.sites s JOIN directory.site_ownerships o ON o.site_id = s.id
WHERE o.user_id = $1 ORDER BY s.name, s.id;

-- name: GetCurrentSiteOwner :one
SELECT o.* FROM directory.site_ownerships o JOIN directory.sites s ON s.id = o.site_id
WHERE s.short_id = $1 AND o.user_id = $2 AND s.visibility <> 'REMOVED'
AND o.address = s.scheme || '://' || s.normalized_host || s.base_path;

-- name: LockCurrentSiteOwner :one
SELECT o.* FROM directory.site_ownerships o JOIN directory.sites s ON s.id = o.site_id
WHERE s.short_id = $1 AND o.user_id = $2 AND s.visibility <> 'REMOVED'
AND o.address = s.scheme || '://' || s.normalized_host || s.base_path
FOR UPDATE OF s, o;

-- name: LockPendingCreateAuditForHost :one
SELECT * FROM directory.site_audits WHERE action = 'CREATE' AND status = 'PENDING'
AND proposed_snapshot ->> 'normalized_host' = sqlc.arg(host)::text FOR UPDATE;

-- name: LockOwnerSubmissionHost :exec
SELECT pg_advisory_xact_lock(hashtextextended('owner-submission:' || sqlc.arg(host)::text, 0));

-- name: AddOwnerFriendRequest :one
INSERT INTO directory.owner_friend_link_requests (audit_id, source_site_id, user_id, ownership_id, notify_by_email)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (audit_id, source_site_id, ownership_id) DO UPDATE SET status = 'PENDING', notify_by_email = EXCLUDED.notify_by_email
RETURNING *;

-- name: ListAccountAudits :many
SELECT a.* FROM directory.site_audits a WHERE a.submitter_user_id = $1 OR EXISTS (
SELECT 1 FROM directory.owner_friend_link_requests r WHERE r.audit_id = a.id AND r.user_id = $1)
ORDER BY a.created_at DESC, a.id DESC LIMIT 50 OFFSET sqlc.arg(page_offset);

-- name: CanReadAccountAudit :one
SELECT EXISTS (SELECT 1 FROM directory.site_audits a WHERE a.id = $1 AND (
a.submitter_user_id = $2 OR EXISTS (SELECT 1 FROM directory.owner_friend_link_requests r WHERE r.audit_id = a.id AND r.user_id = $2)))::boolean;

-- name: ListOwnerFriendLinks :many
SELECT s.short_id AS target_short_id, s.name AS target_name, l.target_host::text AS target_host,
l.target_url::text AS target_url, l.link_status::text AS link_status, l.is_reciprocal::boolean AS is_reciprocal
FROM directory.list_owner_friend_links($1, true) l LEFT JOIN directory.sites s ON s.id = l.target_site_id::uuid AND s.visibility = 'VISIBLE';

-- name: GetAccountSite :one
SELECT s.* FROM directory.sites s JOIN directory.site_ownerships o ON o.site_id = s.id
WHERE s.short_id = $1 AND o.user_id = $2;

-- name: LockFriendEndpointSites :many
SELECT s.id FROM directory.sites s WHERE s.short_id IN (sqlc.arg(source_short_id), sqlc.arg(target_short_id))
ORDER BY s.id FOR UPDATE;

-- name: SetRegisteredFriendLink :exec
SELECT directory.upsert_registered_friend_link($1, $2, $3, $4);

-- name: RemoveOwnerFriendLink :one
SELECT directory.remove_owner_friend_link($1, $2)::boolean;

-- name: ListOwnerDecisionRecipients :many
SELECT DISTINCT u.email::text FROM directory.owner_friend_link_requests r
JOIN identity.users u ON u.id = r.user_id
WHERE r.audit_id = $1 AND r.notify_by_email AND r.status IN ('APPLIED', 'REJECTED')
AND u.deleted_at IS NULL AND u.access_status = 'ACTIVE' AND u.email IS NOT NULL;

-- name: HasOwnerAddressProof :one
SELECT EXISTS (
    SELECT 1 FROM directory.site_claims c JOIN directory.site_ownerships o ON o.site_id = c.site_id AND o.user_id = c.user_id
    WHERE c.site_id = $1 AND c.user_id = $2 AND c.address = $3 AND c.status = 'VERIFIED'
    AND c.consumed_at IS NULL AND c.created_at >= o.created_at
)::boolean;

-- name: ListOwnerFriendRequests :many
SELECT r.id, r.audit_id, r.status, COALESCE(a.proposed_snapshot ->> 'name', '')::text AS target_name,
COALESCE((a.proposed_snapshot ->> 'scheme') || '://' || (a.proposed_snapshot ->> 'normalized_host') || (a.proposed_snapshot ->> 'base_path'), '')::text AS target_url
FROM directory.owner_friend_link_requests r JOIN directory.site_audits a ON a.id = r.audit_id
WHERE r.source_site_id = $1 AND r.user_id = $2 AND (r.status = 'PENDING' OR r.id IN (
    SELECT recent.id FROM directory.owner_friend_link_requests recent
    WHERE recent.source_site_id = $1 AND recent.user_id = $2 AND recent.status <> 'PENDING'
    ORDER BY recent.created_at DESC LIMIT 50
)) ORDER BY r.created_at DESC;

-- name: CancelOwnerFriendRequest :one
UPDATE directory.owner_friend_link_requests SET status = 'CANCELLED'
WHERE id = $1 AND source_site_id = $2 AND user_id = $3 AND status = 'PENDING' RETURNING id;

-- name: GetOwnerFriendRequestForAudit :one
SELECT id FROM directory.owner_friend_link_requests WHERE audit_id = $1 AND source_site_id = $2 AND user_id = $3 AND ownership_id = $4;

-- name: GetAccountFriendRequestAudit :one
SELECT audit_id FROM directory.owner_friend_link_requests WHERE id = $1 AND user_id = $2;

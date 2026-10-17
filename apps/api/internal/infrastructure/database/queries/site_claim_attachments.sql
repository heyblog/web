-- name: CreateSiteClaimAttachment :one
INSERT INTO directory.site_claim_attachments (site_id,user_id,address,filename,content_type,size,storage_key,storage_identity)
VALUES (sqlc.arg(site_id)::uuid,sqlc.arg(user_id)::uuid,sqlc.arg(address),sqlc.arg(filename),sqlc.arg(content_type),sqlc.arg(size),sqlc.arg(storage_key),sqlc.arg(storage_identity)) RETURNING id::text;
-- name: SetSiteClaimAttachmentStorageKey :exec
UPDATE directory.site_claim_attachments SET storage_key=sqlc.arg(storage_key) WHERE id=sqlc.arg(id)::uuid;
-- name: GetSiteClaimAttachment :one
SELECT id::text, coalesce(claim_id::text, '')::text AS claim_id, site_id::text, coalesce(user_id::text, '')::text AS user_id, address, filename, content_type, size, sha256, status, storage_key, storage_identity, created_at FROM directory.site_claim_attachments WHERE id=$1::uuid;
-- name: LockSiteClaimAttachment :one
SELECT id::text, coalesce(claim_id::text, '')::text AS claim_id, site_id::text, coalesce(user_id::text, '')::text AS user_id, address, filename, content_type, size, sha256, status, storage_key, storage_identity, created_at FROM directory.site_claim_attachments WHERE id=$1::uuid FOR UPDATE;
-- name: CompleteSiteClaimAttachment :execrows
UPDATE directory.site_claim_attachments SET sha256=sqlc.arg(sha256),status='READY',completed_at=clock_timestamp()
WHERE id=sqlc.arg(id)::uuid AND user_id=sqlc.arg(user_id)::uuid AND status='UPLOADING'
AND content_type=sqlc.arg(content_type) AND size=sqlc.arg(size);
-- name: DeleteSiteClaimAttachment :execrows
UPDATE directory.site_claim_attachments SET status='CLEANUP_PENDING',cleanup_requested_at=clock_timestamp()
WHERE id=sqlc.arg(id)::uuid AND user_id=sqlc.arg(user_id)::uuid AND status IN ('UPLOADING','READY');
-- name: CountRecentSiteClaimAttachments :one
SELECT count(*) FROM directory.site_claim_attachments WHERE user_id=$1::uuid AND created_at>=clock_timestamp()-interval '24 hours';
-- name: CountManualClaimWindows :one
SELECT count(*) FILTER (WHERE created_at>=clock_timestamp()-interval '24 hours') AS recent,
 count(*) FILTER (WHERE status='PENDING') AS pending FROM directory.site_claims WHERE user_id=$1::uuid AND method='MANUAL';
-- name: LockSiteClaimUser :one
SELECT id FROM identity.users WHERE id=$1::uuid AND deleted_at IS NULL AND access_status='ACTIVE' FOR UPDATE;
-- name: HasRecentRejectedManualClaim :one
SELECT EXISTS (SELECT 1 FROM directory.site_claims WHERE site_id=sqlc.arg(site_id)::uuid AND user_id=sqlc.arg(user_id)::uuid AND method='MANUAL' AND status='REJECTED' AND reviewed_at>=clock_timestamp()-interval '24 hours');
-- name: BindSiteClaimAttachment :execrows
UPDATE directory.site_claim_attachments SET claim_id=sqlc.arg(claim_id)::uuid,status='BOUND',bound_at=clock_timestamp()
WHERE id=sqlc.arg(id)::uuid AND user_id=sqlc.arg(user_id)::uuid AND site_id=sqlc.arg(site_id)::uuid AND address=sqlc.arg(address) AND status='READY' AND claim_id IS NULL AND created_at>clock_timestamp()-interval '24 hours';
-- name: ValidateSiteClaimAttachments :many
SELECT id::text,size FROM directory.site_claim_attachments
WHERE user_id=sqlc.arg(user_id)::uuid AND site_id=sqlc.arg(site_id)::uuid AND address=sqlc.arg(address)
AND status='READY' AND claim_id IS NULL AND created_at>clock_timestamp()-interval '24 hours'
AND id=ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR UPDATE;
-- name: ListSiteClaimAttachmentMetadata :many
SELECT id::text, coalesce(claim_id::text, '')::text AS claim_id, site_id::text, coalesce(user_id::text, '')::text AS user_id, address, filename, content_type, size, sha256, status, storage_key, storage_identity, created_at FROM directory.site_claim_attachments WHERE claim_id=$1::uuid ORDER BY created_at,id;
-- name: ExpireSiteClaimAttachments :exec
UPDATE directory.site_claim_attachments SET status='CLEANUP_PENDING',cleanup_requested_at=clock_timestamp()
WHERE (status='UPLOADING' AND created_at<=clock_timestamp()-interval '15 minutes')
OR (status='READY' AND created_at<=clock_timestamp()-interval '24 hours');
-- name: ListSiteClaimAttachmentCleanup :many
SELECT id::text,status,storage_key,storage_identity,
 (upload_cleaned_at IS NULL)::boolean AS cleanup_upload
FROM directory.site_claim_attachments
WHERE created_at<clock_timestamp()-interval '15 minutes' AND (
 (upload_cleaned_at IS NULL AND status<>'DELETED') OR status='CLEANUP_PENDING'
 OR (status='BOUND' AND archive_at IS NOT NULL AND archive_at<=clock_timestamp())
 OR (status='ARCHIVED' AND purge_at IS NOT NULL AND purge_at<=clock_timestamp()))
ORDER BY created_at,id LIMIT 100;
-- name: MarkSiteClaimAttachmentUploadCleaned :exec
UPDATE directory.site_claim_attachments SET upload_cleaned_at=clock_timestamp() WHERE id=$1::uuid;
-- name: MarkSiteClaimAttachmentArchived :execrows
UPDATE directory.site_claim_attachments SET status='ARCHIVED'
WHERE id=$1::uuid AND status='BOUND' AND archive_at IS NOT NULL AND archive_at<=clock_timestamp();
-- name: MarkSiteClaimAttachmentDeleted :execrows
UPDATE directory.site_claim_attachments SET status='DELETED'
WHERE id=$1::uuid AND status IN ('CLEANUP_PENDING','ARCHIVED');
-- name: GetSiteClaimReviewedAt :one
SELECT reviewed_at FROM directory.site_claims WHERE id=$1::uuid;

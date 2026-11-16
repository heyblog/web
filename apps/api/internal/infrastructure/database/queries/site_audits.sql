-- name: CreateSiteAudit :one
INSERT INTO directory.site_audits (
    lookup_secret_hash,
    action,
    site_id,
    base_revision,
    base_snapshot,
    proposed_snapshot,
    request_reason,
    submitter_name,
    submitter_email,
    notify_by_email,
    submitter_user_id, source_channel, source_site_id, ownership_id
) VALUES (sqlc.arg(lookup_secret_hash), sqlc.arg(action), sqlc.narg(site_id), sqlc.narg(base_revision),
    sqlc.narg(base_snapshot), sqlc.arg(proposed_snapshot), sqlc.arg(request_reason), sqlc.narg(submitter_name),
    sqlc.narg(submitter_email), sqlc.arg(notify_by_email), sqlc.narg(submitter_user_id),
    COALESCE(NULLIF(sqlc.arg(source_channel)::text, ''), 'ANONYMOUS'), sqlc.narg(source_site_id), sqlc.narg(ownership_id))
RETURNING *;

-- name: GetSiteAuditByLookupHash :one
SELECT *
  FROM directory.site_audits
 WHERE lookup_secret_hash = $1;

-- name: GetSiteAuditByID :one
SELECT *
  FROM directory.site_audits
 WHERE id = $1;

-- name: LockSiteAuditByID :one
SELECT *
  FROM directory.site_audits
 WHERE id = $1
 FOR UPDATE;

-- name: ListSiteAuditsForManagement :many
SELECT id, action, status, site_id, submitter_name, submitter_email,
       submitter_user_id, source_channel, source_site_id,
       proposed_snapshot,
       reviewed_by, reviewed_at, created_at, updated_at
  FROM directory.site_audits
 WHERE (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
   AND (sqlc.narg(action)::text IS NULL OR action = sqlc.narg(action))
 ORDER BY created_at DESC, id DESC
 LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountSiteAuditsForManagement :one
SELECT count(*)::bigint
  FROM directory.site_audits
 WHERE (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
   AND (sqlc.narg(action)::text IS NULL OR action = sqlc.narg(action));

-- name: ListSitesForManagement :many
SELECT site.short_id,
       site.custom_id,
       site.name,
       site.scheme,
       site.normalized_host,
       site.base_path,
       site.access_scope,
       site.visibility,
       site.visibility_reason,
       site.revision,
       site.joined_at,
       site.updated_at,
       pending.id AS pending_audit_id,
       pending.action AS pending_audit_action
  FROM directory.sites AS site
  LEFT JOIN directory.site_audits AS pending
    ON pending.site_id = site.id AND pending.status = 'PENDING'
 WHERE (sqlc.arg(visibility)::text = '' OR site.visibility = sqlc.arg(visibility))
   AND (
       sqlc.arg(query_text)::text = ''
       OR site.name ILIKE '%' || sqlc.arg(query_text)::text || '%'
       OR site.normalized_host ILIKE '%' || sqlc.arg(query_text)::text || '%'
       OR site.short_id = sqlc.arg(query_text)::text
       OR site.custom_id = sqlc.arg(query_text)::text
   )
 ORDER BY site.updated_at DESC, site.id DESC
 LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountSitesForManagement :one
SELECT count(*)::bigint
  FROM directory.sites AS site
 WHERE (sqlc.arg(visibility)::text = '' OR site.visibility = sqlc.arg(visibility))
   AND (
       sqlc.arg(query_text)::text = ''
       OR site.name ILIKE '%' || sqlc.arg(query_text)::text || '%'
       OR site.normalized_host ILIKE '%' || sqlc.arg(query_text)::text || '%'
       OR site.short_id = sqlc.arg(query_text)::text
       OR site.custom_id = sqlc.arg(query_text)::text
   );

-- name: SaveSiteAuditReviewDraft :one
UPDATE directory.site_audits
   SET review_draft_snapshot = sqlc.arg(review_draft_snapshot),
       review_draft_revision = review_draft_revision + 1,
       review_draft_updated_by = sqlc.arg(review_draft_updated_by),
       review_draft_updated_at = clock_timestamp()
 WHERE id = sqlc.arg(id)
   AND status = 'PENDING'
   AND action IN ('CREATE', 'UPDATE')
   AND review_draft_revision = sqlc.arg(expected_review_draft_revision)
RETURNING *;

-- name: DiscardSiteAuditReviewDraft :one
UPDATE directory.site_audits
   SET review_draft_snapshot = NULL,
       review_draft_revision = review_draft_revision + 1,
       review_draft_updated_by = sqlc.arg(review_draft_updated_by),
       review_draft_updated_at = clock_timestamp()
 WHERE id = sqlc.arg(id)
   AND status = 'PENDING'
   AND action IN ('CREATE', 'UPDATE')
   AND review_draft_revision = sqlc.arg(expected_review_draft_revision)
RETURNING *;

-- name: ApproveSiteAudit :one
UPDATE directory.site_audits
   SET status = 'APPROVED',
       site_id = COALESCE(sqlc.narg(site_id), site_id),
       final_snapshot = sqlc.arg(final_snapshot),
       reviewer_comment = sqlc.narg(reviewer_comment),
       reviewed_by = sqlc.arg(reviewed_by),
       reviewed_at = clock_timestamp()
 WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: RejectSiteAudit :one
UPDATE directory.site_audits
   SET status = 'REJECTED',
       reviewer_comment = sqlc.arg(reviewer_comment),
       reviewed_by = sqlc.arg(reviewed_by),
       reviewed_at = clock_timestamp()
 WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

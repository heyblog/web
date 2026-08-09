-- name: CreateAnnouncement :one
INSERT INTO content.announcements (
    kind,
    title,
    body_markdown,
    priority,
    action_type,
    action_label,
    action_path,
    action_external_url,
    starts_at,
    ends_at,
    created_by,
    updated_by
) VALUES (
    sqlc.arg(kind),
    sqlc.arg(title),
    sqlc.narg(body_markdown),
    sqlc.arg(priority),
    sqlc.arg(action_type),
    sqlc.narg(action_label),
    sqlc.narg(action_path),
    sqlc.narg(action_external_url),
    sqlc.narg(starts_at),
    sqlc.narg(ends_at),
    sqlc.arg(actor_id),
    sqlc.arg(actor_id)
)
RETURNING *;

-- name: GetAnnouncementByID :one
SELECT * FROM content.announcements WHERE id = $1;

-- name: UpdateAnnouncement :one
UPDATE content.announcements
   SET kind = sqlc.arg(kind),
       title = sqlc.arg(title),
       body_markdown = sqlc.narg(body_markdown),
       priority = sqlc.arg(priority),
       action_type = sqlc.arg(action_type),
       action_label = sqlc.narg(action_label),
       action_path = sqlc.narg(action_path),
       action_external_url = sqlc.narg(action_external_url),
       starts_at = sqlc.narg(starts_at),
       ends_at = sqlc.narg(ends_at),
       updated_by = sqlc.arg(actor_id)
 WHERE id = sqlc.arg(id)
   AND row_version = sqlc.arg(expected_row_version)
RETURNING *;

-- name: PublishAnnouncement :one
UPDATE content.announcements
   SET status = 'PUBLISHED',
       starts_at = sqlc.arg(starts_at),
       ends_at = sqlc.narg(ends_at),
       published_at = clock_timestamp(),
       published_by = sqlc.arg(actor_id),
       updated_by = sqlc.arg(actor_id)
 WHERE id = sqlc.arg(id)
   AND row_version = sqlc.arg(expected_row_version)
   AND status = 'DRAFT'
RETURNING *;

-- name: ArchiveAnnouncement :one
UPDATE content.announcements
   SET status = 'ARCHIVED',
       archived_at = clock_timestamp(),
       archived_by = sqlc.arg(actor_id),
       updated_by = sqlc.arg(actor_id)
 WHERE id = sqlc.arg(id)
   AND row_version = sqlc.arg(expected_row_version)
   AND status = 'PUBLISHED'
RETURNING *;

-- name: DeleteDraftAnnouncement :execrows
DELETE FROM content.announcements
 WHERE id = $1 AND status = 'DRAFT';

-- name: ListActiveMainAnnouncements :many
SELECT *
  FROM content.announcements
 WHERE kind = 'MAIN'
   AND status = 'PUBLISHED'
   AND starts_at <= clock_timestamp()
   AND (ends_at IS NULL OR ends_at > clock_timestamp())
 ORDER BY priority DESC, starts_at DESC, id DESC;

-- name: GetActiveBannerAnnouncement :one
SELECT *
  FROM content.announcements
 WHERE kind = 'BANNER'
   AND status = 'PUBLISHED'
   AND starts_at <= clock_timestamp()
   AND (ends_at IS NULL OR ends_at > clock_timestamp())
 ORDER BY starts_at DESC, id DESC
 LIMIT 1;

-- name: ListPublicAnnouncementArchive :many
SELECT *
  FROM content.announcements
 WHERE kind = 'MAIN'
   AND starts_at <= clock_timestamp()
   AND (
       status = 'PUBLISHED'
       OR (status = 'ARCHIVED' AND archived_at > starts_at)
   )
 ORDER BY starts_at DESC, id DESC
 LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountAnnouncementsForManagement :one
SELECT count(*)::bigint
  FROM content.announcements
 WHERE (sqlc.narg(filter_kind)::text IS NULL OR kind = sqlc.narg(filter_kind))
   AND (sqlc.narg(filter_status)::text IS NULL OR status = sqlc.narg(filter_status));

-- name: ListAnnouncementsForManagement :many
SELECT announcement.*,
       CASE
           WHEN status = 'DRAFT' THEN 'DRAFT'
           WHEN status = 'ARCHIVED' THEN 'ARCHIVED'
           WHEN starts_at > clock_timestamp() THEN 'SCHEDULED'
           WHEN ends_at IS NOT NULL AND ends_at <= clock_timestamp() THEN 'EXPIRED'
           ELSE 'ACTIVE'
       END::text AS effective_status
  FROM content.announcements AS announcement
 WHERE (sqlc.narg(filter_kind)::text IS NULL OR kind = sqlc.narg(filter_kind))
   AND (sqlc.narg(filter_status)::text IS NULL OR status = sqlc.narg(filter_status))
 ORDER BY updated_at DESC, id DESC
 LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: ListAnnouncementRevisions :many
SELECT *
  FROM content.announcement_revisions
 WHERE announcement_id = $1
 ORDER BY revision DESC;

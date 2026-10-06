-- name: ListSitemapSites :many
SELECT id, short_id
  FROM directory.sites
 WHERE visibility = 'VISIBLE'
   AND (sqlc.narg(after_id)::uuid IS NULL OR id > sqlc.narg(after_id)::uuid)
 ORDER BY id ASC
 LIMIT 1001;

-- name: ListSitemapAnnouncements :many
SELECT id, starts_at, published_at, updated_at
  FROM content.announcements
 WHERE kind = 'MAIN' AND starts_at <= clock_timestamp()
   AND (status = 'PUBLISHED' OR (status = 'ARCHIVED' AND archived_at > starts_at))
   AND (sqlc.narg(after_id)::uuid IS NULL OR id > sqlc.narg(after_id)::uuid)
 ORDER BY id ASC
 LIMIT 1001;

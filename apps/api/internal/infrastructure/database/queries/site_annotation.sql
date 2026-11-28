-- name: ListAnnotationSites :many
SELECT sqlc.embed(site), COALESCE(pending.id::text, '')::text AS pending_audit_id
  FROM directory.sites AS site
  LEFT JOIN LATERAL (
      SELECT audit.id
        FROM directory.site_audits AS audit
       WHERE audit.status = 'PENDING'
         AND (audit.site_id = site.id OR (
             audit.action = 'CREATE' AND audit.proposed_snapshot->>'normalized_host' = site.normalized_host
         ))
       ORDER BY audit.created_at, audit.id LIMIT 1
  ) AS pending ON true
 WHERE (sqlc.arg(blog_status)::text = 'ALL' OR site.blog_status = sqlc.arg(blog_status))
   AND (sqlc.arg(query_text)::text = ''
        OR strpos(lower(site.name), lower(sqlc.arg(query_text))) > 0
        OR strpos(site.normalized_host, lower(sqlc.arg(query_text))) > 0
        OR site.short_id = sqlc.arg(query_text) OR site.custom_id = sqlc.arg(query_text))
 ORDER BY site.updated_at DESC, site.id DESC
 LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountAnnotationSites :one
SELECT count(*)::bigint
  FROM directory.sites AS site
 WHERE (sqlc.arg(blog_status)::text = 'ALL' OR site.blog_status = sqlc.arg(blog_status))
   AND (sqlc.arg(query_text)::text = ''
        OR strpos(lower(site.name), lower(sqlc.arg(query_text))) > 0
        OR strpos(site.normalized_host, lower(sqlc.arg(query_text))) > 0
        OR site.short_id = sqlc.arg(query_text) OR site.custom_id = sqlc.arg(query_text));

-- name: AnnotationPendingAudit :one
SELECT COALESCE((
    SELECT audit.id::text
      FROM directory.site_audits AS audit
     WHERE audit.status = 'PENDING'
       AND (audit.site_id = sqlc.arg(site_id)::uuid OR (
           audit.action = 'CREATE' AND audit.proposed_snapshot->>'normalized_host' = sqlc.arg(host)::text
       ))
     ORDER BY audit.created_at, audit.id LIMIT 1
), '')::text;

-- name: UpdateSiteAnnotation :one
UPDATE directory.sites
   SET name = sqlc.arg(name),
       scheme = sqlc.arg(scheme),
       normalized_host = sqlc.arg(normalized_host),
       base_path = sqlc.arg(base_path),
       summary = sqlc.arg(summary),
       blog_status = sqlc.arg(blog_status),
       tag_cascade_id = sqlc.arg(tag_cascade_id),
       revision = revision + 1
 WHERE id = sqlc.arg(site_id) AND revision = sqlc.arg(revision)
RETURNING *;

-- name: ListRecordedOutgoingLinks :many
SELECT link.target_host::text AS target_host, link.target_url::text AS target_url,
       link.link_status::text AS link_status, link.updated_at_ms::bigint AS updated_at_ms
  FROM directory.list_recorded_friend_links(sqlc.arg(site_id)::uuid) AS link
 ORDER BY link.target_host;

-- name: ListRecordedIncomingLinks :many
SELECT source.short_id AS source_short_id, source.name AS source_name,
       (source.scheme || '://' || source.normalized_host || source.base_path)::text AS source_url,
       link.link_status::text AS link_status
  FROM directory.list_recorded_incoming_links(sqlc.arg(host)::text) AS link
  JOIN directory.sites AS source ON source.id = link.source_site_id
 ORDER BY source.normalized_host, source.id;

-- name: InsertSiteAnnotationHistory :exec
INSERT INTO directory.site_annotation_history (
    site_id, actor_id, site_revision, source, site_audit_id, reason, before_snapshot, after_snapshot
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListSiteAnnotationHistory :many
SELECT history.*, actor.display_name AS actor_name
  FROM directory.site_annotation_history AS history
  JOIN identity.users AS actor ON actor.id = history.actor_id
 WHERE site_id = sqlc.arg(site_id)
 ORDER BY site_revision DESC
 LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountSiteAnnotationHistory :one
SELECT count(*)::bigint FROM directory.site_annotation_history WHERE site_id = $1;

-- name: LatestNonBlogClassificationRevision :one
SELECT COALESCE(MAX(site_revision), 0)::bigint
  FROM directory.site_annotation_history
 WHERE site_id = $1 AND source = 'ANNOTATION'
   AND after_snapshot->>'blog_status' = 'NON_BLOG'
   AND before_snapshot->>'blog_status' IS DISTINCT FROM 'NON_BLOG';

-- name: GetAnnotationStatusCounts :one
SELECT count(*)::bigint AS total,
       count(*) FILTER (WHERE blog_status = 'UNREVIEWED')::bigint AS unreviewed,
       count(*) FILTER (WHERE blog_status = 'BLOG')::bigint AS blog,
       count(*) FILTER (WHERE blog_status = 'NON_BLOG')::bigint AS non_blog
  FROM directory.sites;

-- name: GetActorAnnotationCounts :one
SELECT count(DISTINCT site_id)::bigint AS annotated_sites,
       count(DISTINCT site_id) FILTER (
           WHERE created_at >= (date_trunc('day', now() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai')
       )::bigint AS today_annotated_sites
  FROM directory.site_annotation_history
 WHERE actor_id = sqlc.arg(actor_id)::uuid AND source = 'ANNOTATION';

-- name: PickNextUnreviewedAnnotationSite :one
SELECT short_id
  FROM directory.sites
 WHERE blog_status = 'UNREVIEWED'
   AND (sqlc.arg(exclude_short_id)::text = '' OR short_id <> sqlc.arg(exclude_short_id))
 ORDER BY random()
 LIMIT 1;

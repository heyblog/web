-- name: UpsertFriendLink :exec
SELECT directory.upsert_friend_link($1, $2, $3, $4);

-- name: ListFriendLinks :many
SELECT link.target_site_id::uuid AS target_site_id,
       link.target_host::text AS target_host,
       link.target_url::text AS target_url,
       link.link_status::text AS link_status,
       link.is_reciprocal::boolean AS is_reciprocal,
       link.created_at_ms::bigint AS created_at_ms,
       link.updated_at_ms::bigint AS updated_at_ms
  FROM directory.list_friend_links($1, $2) AS link;

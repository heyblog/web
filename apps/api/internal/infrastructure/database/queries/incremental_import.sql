-- name: LockIncrementalSites :exec
LOCK TABLE directory.sites IN SHARE ROW EXCLUSIVE MODE;

-- name: LockIncrementalHosts :exec
SELECT pg_advisory_xact_lock(hashtextextended('site-ref:' || hosts.host, 0))
  FROM (SELECT host FROM unnest(sqlc.arg(hosts)::text[]) AS host ORDER BY host) AS hosts;

-- name: ListIncrementalSites :many
SELECT id, short_id, normalized_host, scheme, base_path
  FROM directory.sites ORDER BY normalized_host;

-- name: ListIncrementalFriendPairs :many
SELECT pairs.source_site_id::uuid AS source_site_id,
       pairs.target_host::text AS target_host
  FROM directory.import_friend_link_pairs() AS pairs;

-- name: InsertIncrementalSource :execrows
INSERT INTO directory.site_sources (source_key, name, is_enabled)
VALUES ($1, $2, true)
ON CONFLICT (source_key) DO NOTHING;

-- name: GetIncrementalSource :one
SELECT id, is_enabled FROM directory.site_sources WHERE source_key = $1;

-- name: InsertIncrementalOrigin :execrows
INSERT INTO directory.site_origins (
    site_id, source_id, external_reference, first_discovered_at, metadata
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (site_id, source_id) DO NOTHING;

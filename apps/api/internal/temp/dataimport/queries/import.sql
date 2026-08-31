-- name: TryAcquireImportLock :one
SELECT pg_try_advisory_xact_lock(hashtextextended(sqlc.arg(lock_name), 0));

-- name: ImportLockCapacity :one
SELECT current_setting('max_locks_per_transaction')::integer;

-- name: DirectoryIsEmpty :one
SELECT NOT EXISTS (
    SELECT 1 FROM directory.sites
    UNION ALL SELECT 1 FROM directory.site_feeds
    UNION ALL SELECT 1 FROM directory.site_resources
    UNION ALL SELECT 1 FROM directory.site_icons
    UNION ALL SELECT 1 FROM directory.tags
    UNION ALL SELECT 1 FROM directory.site_tags
    UNION ALL SELECT 1 FROM directory.software_components
    UNION ALL SELECT 1 FROM directory.software_component_dependencies
    UNION ALL SELECT 1 FROM directory.site_software_components
    UNION ALL SELECT 1 FROM directory.site_sources
    UNION ALL SELECT 1 FROM directory.site_origins
);

-- name: InsertSite :exec
INSERT INTO directory.sites (
    id,
    short_id,
    name,
    scheme,
    normalized_host,
    base_path,
    summary,
    access_scope,
    visibility,
    visibility_reason,
    joined_at,
    updated_at
) VALUES (
    sqlc.arg(id)::uuid,
    sqlc.arg(short_id),
    sqlc.arg(name),
    sqlc.arg(scheme),
    sqlc.arg(normalized_host),
    sqlc.arg(base_path),
    sqlc.arg(summary),
    sqlc.arg(access_scope),
    sqlc.arg(visibility),
    sqlc.narg(visibility_reason)::text,
    sqlc.arg(joined_at)::timestamptz,
    sqlc.arg(updated_at)::timestamptz
);

-- name: InsertFeed :exec
INSERT INTO directory.site_feeds (
    site_id,
    name,
    location_type,
    url_ref,
    external_url,
    url_key,
    format,
    is_enabled,
    is_default
) VALUES (
    sqlc.arg(site_id)::uuid,
    sqlc.arg(name),
    sqlc.arg(location_type),
    sqlc.narg(url_ref)::text,
    sqlc.narg(external_url)::text,
    sqlc.arg(url_key),
    sqlc.arg(format),
    sqlc.arg(is_enabled),
    sqlc.arg(is_default)
);

-- name: InsertResource :exec
INSERT INTO directory.site_resources (
    site_id,
    kind,
    location_type,
    url_ref,
    external_url,
    url_key
) VALUES (
    sqlc.arg(site_id)::uuid,
    sqlc.arg(kind),
    sqlc.arg(location_type),
    sqlc.narg(url_ref)::text,
    sqlc.narg(external_url)::text,
    sqlc.arg(url_key)
);

-- name: InsertTag :exec
INSERT INTO directory.tags (
    id,
    name,
    normalized_name,
    slug,
    description,
    is_enabled
) VALUES (
    sqlc.arg(id)::uuid,
    sqlc.arg(name),
    sqlc.arg(normalized_name),
    sqlc.arg(slug),
    sqlc.arg(description),
    sqlc.arg(is_enabled)
);

-- name: InsertSiteTag :exec
INSERT INTO directory.site_tags (
    site_id,
    tag_id,
    role,
    assignment_source,
    note
) VALUES (
    sqlc.arg(site_id)::uuid,
    sqlc.arg(tag_id)::uuid,
    sqlc.arg(role),
    'IMPORTED',
    sqlc.narg(note)
);

-- name: InsertSoftwareComponent :exec
INSERT INTO directory.software_components (
    id,
    name,
    normalized_name,
    description,
    homepage_url,
    repository_url,
    is_open_source,
    is_enabled
) VALUES (
    sqlc.arg(id)::uuid,
    sqlc.arg(name),
    sqlc.arg(normalized_name),
    sqlc.arg(description),
    sqlc.narg(homepage_url)::text,
    sqlc.narg(repository_url)::text,
    sqlc.arg(is_open_source),
    sqlc.arg(is_enabled)
);

-- name: InsertSoftwareDependency :exec
INSERT INTO directory.software_component_dependencies (
    component_id,
    dependency_component_id,
    role
) VALUES (
    sqlc.arg(component_id)::uuid,
    sqlc.arg(dependency_component_id)::uuid,
    sqlc.arg(role)
);

-- name: InsertSiteSoftwareComponent :exec
INSERT INTO directory.site_software_components (
    site_id,
    component_id,
    role,
    evidence_source,
    first_identified_at,
    last_confirmed_at
) VALUES (
    sqlc.arg(site_id)::uuid,
    sqlc.arg(component_id)::uuid,
    sqlc.arg(role),
    'IMPORTED',
    sqlc.arg(identified_at)::timestamptz,
    sqlc.arg(identified_at)::timestamptz
);

-- name: InsertSource :one
INSERT INTO directory.site_sources (source_key, name, is_enabled)
VALUES (sqlc.arg(source_key), sqlc.arg(name), true)
RETURNING id;

-- name: InsertOrigin :exec
INSERT INTO directory.site_origins (
    site_id,
    source_id,
    external_reference,
    first_discovered_at,
    metadata
) VALUES (
    sqlc.arg(site_id)::uuid,
    sqlc.arg(source_id)::uuid,
    sqlc.arg(external_reference),
    sqlc.arg(first_discovered_at)::timestamptz,
    sqlc.arg(metadata)::jsonb
);

-- name: InsertFriendLinks :exec
SELECT directory.upsert_registered_friend_link(
    link.source_site_id,
    link.target_url,
    link.target_host,
    'ACTIVE'
)
FROM jsonb_to_recordset(sqlc.arg(links)::jsonb) AS link(
    source_site_id uuid,
    target_url text,
    target_host text
);

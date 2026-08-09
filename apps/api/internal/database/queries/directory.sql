-- name: CreateSite :one
INSERT INTO directory.sites (
    short_id,
    custom_id,
    name,
    scheme,
    normalized_host,
    base_path,
    summary,
    access_scope
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetSiteByID :one
SELECT * FROM directory.sites WHERE id = $1;

-- name: GetSiteByShortID :one
SELECT * FROM directory.sites WHERE short_id = $1;

-- name: GetSiteByCustomID :one
SELECT * FROM directory.sites WHERE custom_id = $1;

-- name: GetSiteByHost :one
SELECT * FROM directory.sites WHERE normalized_host = $1;

-- name: ListVisibleSites :many
SELECT *
  FROM directory.sites
 WHERE visibility = 'VISIBLE'
 ORDER BY joined_at DESC, id DESC
 LIMIT $1 OFFSET $2;

-- name: UpdateSiteAddress :one
UPDATE directory.sites
   SET scheme = $2,
       normalized_host = $3,
       base_path = $4
 WHERE id = $1 AND revision = $5
RETURNING *;

-- name: UpdateSiteDirectoryProfile :one
UPDATE directory.sites
   SET custom_id = $2,
       name = $3,
       summary = $4,
       access_scope = $5
 WHERE id = $1 AND revision = $6
RETURNING *;

-- name: SetSiteVisibility :one
UPDATE directory.sites
   SET visibility = $2,
       visibility_reason = $3
 WHERE id = $1 AND revision = $4
RETURNING *;

-- name: UpsertSiteFeed :one
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
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (site_id, url_key) DO UPDATE
   SET name = EXCLUDED.name,
       location_type = EXCLUDED.location_type,
       url_ref = EXCLUDED.url_ref,
       external_url = EXCLUDED.external_url,
       format = EXCLUDED.format,
       is_enabled = EXCLUDED.is_enabled,
       is_default = EXCLUDED.is_default
RETURNING *;

-- name: ListSiteFeeds :many
SELECT * FROM directory.site_feeds WHERE site_id = $1 ORDER BY is_default DESC, created_at, id;

-- name: DeleteSiteFeed :exec
DELETE FROM directory.site_feeds WHERE id = $1 AND site_id = $2;

-- name: UpsertSiteResource :one
INSERT INTO directory.site_resources (
    site_id,
    kind,
    location_type,
    url_ref,
    external_url,
    url_key
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (site_id, kind) DO UPDATE
   SET location_type = EXCLUDED.location_type,
       url_ref = EXCLUDED.url_ref,
       external_url = EXCLUDED.external_url,
       url_key = EXCLUDED.url_key
RETURNING *;

-- name: ListSiteResources :many
SELECT * FROM directory.site_resources WHERE site_id = $1 ORDER BY kind;

-- name: DeleteSiteResource :exec
DELETE FROM directory.site_resources WHERE site_id = $1 AND kind = $2;

-- name: UpsertSiteIcon :one
INSERT INTO directory.site_icons (
    site_id,
    content,
    media_type,
    sha256,
    source_location_type,
    source_url_ref,
    source_external_url,
    fetched_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (site_id) DO UPDATE
   SET content = EXCLUDED.content,
       media_type = EXCLUDED.media_type,
       sha256 = EXCLUDED.sha256,
       source_location_type = EXCLUDED.source_location_type,
       source_url_ref = EXCLUDED.source_url_ref,
       source_external_url = EXCLUDED.source_external_url,
       fetched_at = EXCLUDED.fetched_at
RETURNING *;

-- name: GetSiteIcon :one
SELECT * FROM directory.site_icons WHERE site_id = $1;

-- name: CreateTag :one
INSERT INTO directory.tags (kind, name, normalized_name, slug, description)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListEnabledTags :many
SELECT * FROM directory.tags
 WHERE kind = $1 AND is_enabled AND merged_into_id IS NULL
 ORDER BY name, id;

-- name: AssignSiteTag :one
INSERT INTO directory.site_tags (
    site_id,
    tag_id,
    tag_kind,
    topic_role,
    assignment_source,
    assigned_by,
    note
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (site_id, tag_id) DO UPDATE
   SET topic_role = EXCLUDED.topic_role,
       assignment_source = EXCLUDED.assignment_source,
       assigned_by = EXCLUDED.assigned_by,
       note = EXCLUDED.note
RETURNING *;

-- name: ListSiteTags :many
SELECT assignment.*, tag.name, tag.slug, tag.description
  FROM directory.site_tags AS assignment
  JOIN directory.tags AS tag ON tag.id = assignment.tag_id
 WHERE assignment.site_id = $1
 ORDER BY assignment.tag_kind, assignment.topic_role NULLS LAST, tag.name;

-- name: UnassignSiteTag :exec
DELETE FROM directory.site_tags WHERE site_id = $1 AND tag_id = $2;

-- name: CreateSoftwareComponent :one
INSERT INTO directory.software_components (
    name,
    normalized_name,
    description,
    homepage_url,
    repository_url,
    is_open_source
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListEnabledSoftwareComponents :many
SELECT * FROM directory.software_components WHERE is_enabled ORDER BY name, id;

-- name: GetSoftwareComponentByID :one
SELECT * FROM directory.software_components WHERE id = $1;

-- name: AddSoftwareComponentDependency :one
INSERT INTO directory.software_component_dependencies (
    component_id,
    dependency_component_id,
    role
) VALUES ($1, $2, $3)
RETURNING *;

-- name: ListSoftwareComponentDependencies :many
SELECT dependency_relation.*, dependency.name, dependency.normalized_name,
       dependency.description, dependency.homepage_url, dependency.repository_url,
       dependency.is_open_source, dependency.is_enabled
  FROM directory.software_component_dependencies AS dependency_relation
  JOIN directory.software_components AS dependency
    ON dependency.id = dependency_relation.dependency_component_id
 WHERE dependency_relation.component_id = $1
 ORDER BY dependency_relation.role, dependency.name, dependency.id;

-- name: RemoveSoftwareComponentDependency :exec
DELETE FROM directory.software_component_dependencies
 WHERE component_id = $1 AND dependency_component_id = $2 AND role = $3;

-- name: AssignSiteSoftwareComponent :one
INSERT INTO directory.site_software_components (
    site_id,
    component_id,
    role,
    evidence_source,
    confidence,
    identified_by
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (site_id, component_id, role) DO UPDATE
   SET evidence_source = EXCLUDED.evidence_source,
       confidence = EXCLUDED.confidence,
       identified_by = EXCLUDED.identified_by,
       last_confirmed_at = clock_timestamp()
RETURNING *;

-- name: ListSiteSoftwareComponents :many
SELECT assignment.*, component.name, component.normalized_name,
       component.homepage_url, component.repository_url, component.is_open_source
  FROM directory.site_software_components AS assignment
  JOIN directory.software_components AS component ON component.id = assignment.component_id
 WHERE assignment.site_id = $1
 ORDER BY assignment.role, component.name;

-- name: UnassignSiteSoftwareComponent :exec
DELETE FROM directory.site_software_components
 WHERE site_id = $1 AND component_id = $2 AND role = $3;

-- name: UpsertSiteSource :one
INSERT INTO directory.site_sources (source_key, name, base_url, is_enabled)
VALUES ($1, $2, $3, $4)
ON CONFLICT (source_key) DO UPDATE
   SET name = EXCLUDED.name,
       base_url = EXCLUDED.base_url,
       is_enabled = EXCLUDED.is_enabled
RETURNING *;

-- name: AddSiteOrigin :one
INSERT INTO directory.site_origins (
    site_id,
    source_id,
    external_reference,
    first_discovered_at,
    metadata
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (site_id, source_id) DO UPDATE
   SET external_reference = COALESCE(EXCLUDED.external_reference, directory.site_origins.external_reference),
       first_discovered_at = LEAST(EXCLUDED.first_discovered_at, directory.site_origins.first_discovered_at),
       metadata = directory.site_origins.metadata || EXCLUDED.metadata
RETURNING *;

-- name: ListSiteOrigins :many
SELECT origin.*, source.source_key, source.name, source.base_url
  FROM directory.site_origins AS origin
  JOIN directory.site_sources AS source ON source.id = origin.source_id
 WHERE origin.site_id = $1
 ORDER BY origin.first_discovered_at, source.source_key;

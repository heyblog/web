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

-- name: LockSiteByID :one
SELECT * FROM directory.sites WHERE id = $1 FOR UPDATE;

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

-- name: ListRandomVisibleSites :many
SELECT *
  FROM directory.sites
 WHERE visibility = 'VISIBLE'
 ORDER BY random()
 LIMIT $1;

-- name: CountVisibleSites :one
SELECT count(*)::bigint
  FROM directory.sites
 WHERE visibility = 'VISIBLE';

-- name: CountDirectorySitesByStatus :one
SELECT count(*) FILTER (WHERE site.visibility = 'VISIBLE')::bigint AS normal_count,
       count(*) FILTER (WHERE site.visibility = 'HIDDEN')::bigint AS abnormal_count
  FROM directory.sites AS site
 WHERE site.visibility IN ('VISIBLE', 'HIDDEN')
   AND (
       sqlc.arg(query_text)::text = ''
       OR strpos(
           lower(site.name || ' ' || site.normalized_host || ' ' || site.summary),
           lower(sqlc.arg(query_text)::text)
       ) > 0
   )
   AND (
       cardinality(sqlc.arg(primary_tag_slugs)::text[]) = 0
       OR EXISTS (
           SELECT 1
             FROM directory.site_tags AS assignment
             JOIN directory.tags AS tag ON tag.id = assignment.tag_id
            WHERE assignment.site_id = site.id
              AND assignment.role = 'PRIMARY'
              AND tag.is_enabled
              AND tag.merged_into_id IS NULL
              AND tag.slug = ANY(sqlc.arg(primary_tag_slugs)::text[])
       )
   )
   AND (
       cardinality(sqlc.arg(secondary_tag_slugs)::text[]) = 0
       OR (
           SELECT count(DISTINCT tag.slug)
             FROM directory.site_tags AS assignment
             JOIN directory.tags AS tag ON tag.id = assignment.tag_id
            WHERE assignment.site_id = site.id
              AND assignment.role = 'SECONDARY'
              AND tag.is_enabled
              AND tag.merged_into_id IS NULL
              AND tag.slug = ANY(sqlc.arg(secondary_tag_slugs)::text[])
       ) = cardinality(sqlc.arg(secondary_tag_slugs)::text[])
   )
   AND (
       cardinality(sqlc.arg(warning_slugs)::text[]) = 0
       OR EXISTS (
           SELECT 1
             FROM directory.site_tags AS assignment
             JOIN directory.tags AS tag ON tag.id = assignment.tag_id
            WHERE assignment.site_id = site.id
              AND assignment.role = 'WARNING'
              AND tag.is_enabled
              AND tag.merged_into_id IS NULL
              AND tag.slug = ANY(sqlc.arg(warning_slugs)::text[])
       )
   )
   AND (
       cardinality(sqlc.arg(technology_names)::text[]) = 0
       OR EXISTS (
           SELECT 1
             FROM directory.site_software_components AS assignment
             JOIN directory.software_components AS component ON component.id = assignment.component_id
            WHERE assignment.site_id = site.id
              AND component.is_enabled
              AND component.normalized_name = ANY(sqlc.arg(technology_names)::text[])
       )
   )
   AND (
       cardinality(sqlc.arg(access_scopes)::text[]) = 0
       OR site.access_scope = ANY(sqlc.arg(access_scopes)::text[])
   )
   AND (
       sqlc.arg(feed_mode)::text = 'any'
       OR (
           sqlc.arg(feed_mode)::text = 'with'
           AND EXISTS (
               SELECT 1
                 FROM directory.site_feeds AS feed
                WHERE feed.site_id = site.id AND feed.is_enabled
           )
       )
       OR (
           sqlc.arg(feed_mode)::text = 'without'
           AND NOT EXISTS (
               SELECT 1
                 FROM directory.site_feeds AS feed
                WHERE feed.site_id = site.id AND feed.is_enabled
           )
       )
   );

-- name: ListDirectorySites :many
SELECT site.*
  FROM directory.sites AS site
 WHERE site.visibility = sqlc.arg(site_visibility)::text
   AND (
       sqlc.arg(query_text)::text = ''
       OR strpos(
           lower(site.name || ' ' || site.normalized_host || ' ' || site.summary),
           lower(sqlc.arg(query_text)::text)
       ) > 0
   )
   AND (
       cardinality(sqlc.arg(primary_tag_slugs)::text[]) = 0
       OR EXISTS (
           SELECT 1
             FROM directory.site_tags AS assignment
             JOIN directory.tags AS tag ON tag.id = assignment.tag_id
            WHERE assignment.site_id = site.id
              AND assignment.role = 'PRIMARY'
              AND tag.is_enabled
              AND tag.merged_into_id IS NULL
              AND tag.slug = ANY(sqlc.arg(primary_tag_slugs)::text[])
       )
   )
   AND (
       cardinality(sqlc.arg(secondary_tag_slugs)::text[]) = 0
       OR (
           SELECT count(DISTINCT tag.slug)
             FROM directory.site_tags AS assignment
             JOIN directory.tags AS tag ON tag.id = assignment.tag_id
            WHERE assignment.site_id = site.id
              AND assignment.role = 'SECONDARY'
              AND tag.is_enabled
              AND tag.merged_into_id IS NULL
              AND tag.slug = ANY(sqlc.arg(secondary_tag_slugs)::text[])
       ) = cardinality(sqlc.arg(secondary_tag_slugs)::text[])
   )
   AND (
       cardinality(sqlc.arg(warning_slugs)::text[]) = 0
       OR EXISTS (
           SELECT 1
             FROM directory.site_tags AS assignment
             JOIN directory.tags AS tag ON tag.id = assignment.tag_id
            WHERE assignment.site_id = site.id
              AND assignment.role = 'WARNING'
              AND tag.is_enabled
              AND tag.merged_into_id IS NULL
              AND tag.slug = ANY(sqlc.arg(warning_slugs)::text[])
       )
   )
   AND (
       cardinality(sqlc.arg(technology_names)::text[]) = 0
       OR EXISTS (
           SELECT 1
             FROM directory.site_software_components AS assignment
             JOIN directory.software_components AS component ON component.id = assignment.component_id
            WHERE assignment.site_id = site.id
              AND component.is_enabled
              AND component.normalized_name = ANY(sqlc.arg(technology_names)::text[])
       )
   )
   AND (
       cardinality(sqlc.arg(access_scopes)::text[]) = 0
       OR site.access_scope = ANY(sqlc.arg(access_scopes)::text[])
   )
   AND (
       sqlc.arg(feed_mode)::text = 'any'
       OR (
           sqlc.arg(feed_mode)::text = 'with'
           AND EXISTS (
               SELECT 1
                 FROM directory.site_feeds AS feed
                WHERE feed.site_id = site.id AND feed.is_enabled
           )
       )
       OR (
           sqlc.arg(feed_mode)::text = 'without'
           AND NOT EXISTS (
               SELECT 1
                 FROM directory.site_feeds AS feed
                WHERE feed.site_id = site.id AND feed.is_enabled
           )
       )
   )
 ORDER BY
       CASE WHEN sqlc.arg(sort_mode)::text = 'random'
           THEN md5(sqlc.arg(seed)::text || ':' || site.short_id)
       END,
       CASE WHEN sqlc.arg(sort_mode)::text = 'joined' AND sqlc.arg(sort_order)::text = 'desc'
           THEN site.joined_at
       END DESC,
       CASE WHEN sqlc.arg(sort_mode)::text = 'joined' AND sqlc.arg(sort_order)::text = 'asc'
           THEN site.joined_at
       END ASC,
       CASE WHEN sqlc.arg(sort_mode)::text = 'updated' AND sqlc.arg(sort_order)::text = 'desc'
           THEN site.updated_at
       END DESC,
       CASE WHEN sqlc.arg(sort_mode)::text = 'updated' AND sqlc.arg(sort_order)::text = 'asc'
           THEN site.updated_at
       END ASC,
       site.short_id
 LIMIT sqlc.arg(page_limit)::integer
OFFSET sqlc.arg(page_offset)::integer;

-- name: ListDirectoryTagOptions :many
SELECT tag.name, tag.slug, assignment.role,
       count(DISTINCT assignment.site_id) FILTER (
           WHERE site.visibility = 'VISIBLE'
       )::bigint AS normal_count,
       count(DISTINCT assignment.site_id) FILTER (
           WHERE site.visibility = 'HIDDEN'
       )::bigint AS abnormal_count
  FROM directory.site_tags AS assignment
  JOIN directory.tags AS tag ON tag.id = assignment.tag_id
  JOIN directory.sites AS site ON site.id = assignment.site_id
 WHERE site.visibility IN ('VISIBLE', 'HIDDEN')
   AND tag.is_enabled
   AND tag.merged_into_id IS NULL
 GROUP BY tag.id, tag.name, tag.slug, assignment.role
 ORDER BY assignment.role, normal_count DESC, abnormal_count DESC, tag.name, tag.slug;

-- name: ListDirectoryTechnologyOptions :many
SELECT component.name, component.normalized_name,
       count(DISTINCT assignment.site_id) FILTER (
           WHERE site.visibility = 'VISIBLE'
       )::bigint AS normal_count,
       count(DISTINCT assignment.site_id) FILTER (
           WHERE site.visibility = 'HIDDEN'
       )::bigint AS abnormal_count
  FROM directory.site_software_components AS assignment
  JOIN directory.software_components AS component ON component.id = assignment.component_id
  JOIN directory.sites AS site ON site.id = assignment.site_id
 WHERE site.visibility IN ('VISIBLE', 'HIDDEN') AND component.is_enabled
 GROUP BY component.id, component.name, component.normalized_name
 ORDER BY normal_count DESC, abnormal_count DESC, component.name, component.normalized_name;

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

-- name: ApplySiteSnapshot :one
UPDATE directory.sites
   SET name = $2,
       scheme = $3,
       normalized_host = $4,
       base_path = $5,
       summary = $6,
       access_scope = $7,
       visibility = $8,
       visibility_reason = $9
 WHERE id = $1 AND revision = $10
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

-- name: ListPublicSiteFeeds :many
SELECT *
  FROM directory.site_feeds
 WHERE site_id = $1 AND is_enabled
 ORDER BY is_default DESC, created_at, id;

-- name: ListDefaultPublicSiteFeedsBySiteIDs :many
SELECT *
  FROM directory.site_feeds
 WHERE site_id = ANY(sqlc.arg(site_ids)::uuid[])
   AND is_enabled
   AND is_default
 ORDER BY site_id;

-- name: DeleteSiteFeed :exec
DELETE FROM directory.site_feeds WHERE id = $1 AND site_id = $2;

-- name: DeleteSiteFeeds :exec
DELETE FROM directory.site_feeds WHERE site_id = $1;

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

-- name: ListPublicSitemapsBySiteIDs :many
SELECT *
  FROM directory.site_resources
 WHERE site_id = ANY(sqlc.arg(site_ids)::uuid[])
   AND kind = 'SITEMAP'
 ORDER BY site_id;

-- name: DeleteSiteResource :exec
DELETE FROM directory.site_resources WHERE site_id = $1 AND kind = $2;

-- name: DeleteSiteResources :exec
DELETE FROM directory.site_resources WHERE site_id = $1;

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
INSERT INTO directory.tags (name, normalized_name, slug, description)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListEnabledTags :many
SELECT * FROM directory.tags
 WHERE is_enabled AND merged_into_id IS NULL
 ORDER BY name, id;

-- name: GetTagByNormalizedName :one
SELECT * FROM directory.tags WHERE normalized_name = $1 AND merged_into_id IS NULL;

-- name: AssignSiteTag :one
INSERT INTO directory.site_tags (
    site_id,
    tag_id,
    role,
    assignment_source,
    note
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (site_id, tag_id) DO UPDATE
   SET role = EXCLUDED.role,
       assignment_source = EXCLUDED.assignment_source,
       note = EXCLUDED.note
RETURNING *;

-- name: ListSiteTags :many
SELECT assignment.*, tag.name, tag.slug, tag.description
  FROM directory.site_tags AS assignment
  JOIN directory.tags AS tag ON tag.id = assignment.tag_id
 WHERE assignment.site_id = $1
 ORDER BY assignment.role, tag.name;

-- name: ListPublicSiteTags :many
SELECT assignment.*, tag.name, tag.slug, tag.description
  FROM directory.site_tags AS assignment
  JOIN directory.tags AS tag ON tag.id = assignment.tag_id
 WHERE assignment.site_id = $1
   AND tag.is_enabled
   AND tag.merged_into_id IS NULL
 ORDER BY assignment.role, tag.name;

-- name: ListPublicSiteTagsBySiteIDs :many
SELECT assignment.*, tag.name, tag.slug, tag.description
  FROM directory.site_tags AS assignment
  JOIN directory.tags AS tag ON tag.id = assignment.tag_id
 WHERE assignment.site_id = ANY(sqlc.arg(site_ids)::uuid[])
   AND tag.is_enabled
   AND tag.merged_into_id IS NULL
 ORDER BY assignment.site_id, assignment.role, tag.name;

-- name: UnassignSiteTag :exec
DELETE FROM directory.site_tags WHERE site_id = $1 AND tag_id = $2;

-- name: UnassignAllSiteTags :exec
DELETE FROM directory.site_tags WHERE site_id = $1;

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

-- name: GetSoftwareComponentByNormalizedName :one
SELECT * FROM directory.software_components WHERE normalized_name = $1;

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

-- name: ListEnabledSoftwareComponentDependencies :many
SELECT dependency_relation.component_id,
       dependency_relation.dependency_component_id,
       dependency_relation.role
  FROM directory.software_component_dependencies AS dependency_relation
  JOIN directory.software_components AS component
    ON component.id = dependency_relation.component_id
  JOIN directory.software_components AS dependency
    ON dependency.id = dependency_relation.dependency_component_id
 WHERE component.is_enabled AND dependency.is_enabled
 ORDER BY dependency_relation.component_id, dependency_relation.role,
          dependency.name, dependency_relation.dependency_component_id;

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

-- name: ListPublicSiteSoftwareComponents :many
SELECT assignment.*, component.name, component.normalized_name,
       component.homepage_url, component.repository_url, component.is_open_source
  FROM directory.site_software_components AS assignment
  JOIN directory.software_components AS component ON component.id = assignment.component_id
 WHERE assignment.site_id = $1 AND component.is_enabled
 ORDER BY assignment.role, component.name;

-- name: UnassignSiteSoftwareComponent :exec
DELETE FROM directory.site_software_components
 WHERE site_id = $1 AND component_id = $2 AND role = $3;

-- name: UnassignAllSiteSoftwareComponents :exec
DELETE FROM directory.site_software_components WHERE site_id = $1;

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

-- name: GetSiteSourceByKey :one
SELECT * FROM directory.site_sources WHERE source_key = $1;

-- name: SearchSitesForSubmission :many
SELECT *
  FROM directory.sites
 WHERE name ILIKE '%' || sqlc.arg(query)::text || '%'
    OR normalized_host ILIKE '%' || sqlc.arg(query)::text || '%'
    OR short_id = sqlc.arg(query)::text
 ORDER BY visibility, name, id
 LIMIT 12;

-- name: GetSlugModelSetting :one
SELECT model_id, revision FROM content.system_ai_settings WHERE singleton = true;

-- name: SaveSlugModelSetting :one
INSERT INTO content.system_ai_settings (singleton, model_id, revision)
SELECT true, sqlc.arg(model_id)::text, 1 WHERE sqlc.arg(expected_revision)::bigint = 0
ON CONFLICT (singleton) DO UPDATE
SET model_id = EXCLUDED.model_id, revision = system_ai_settings.revision + 1, updated_at = now()
WHERE system_ai_settings.revision = sqlc.arg(expected_revision)::bigint
RETURNING model_id, revision;

-- name: UpdateSlugModelSetting :one
UPDATE content.system_ai_settings
SET model_id = sqlc.arg(model_id)::text, revision = revision + 1, updated_at = now()
WHERE singleton = true AND revision = sqlc.arg(expected_revision)::bigint
RETURNING model_id, revision;

-- name: GetGeneratedSlug :one
SELECT slug FROM directory.slug_generation_cache WHERE cache_key = $1;

-- name: CacheGeneratedSlug :exec
INSERT INTO directory.slug_generation_cache (cache_key, slug)
VALUES ($1, $2) ON CONFLICT (cache_key) DO NOTHING;

-- name: SlugCandidateOccupied :one
SELECT EXISTS (
    SELECT 1 FROM directory.tags WHERE slug = sqlc.arg(slug)::text AND id::text <> sqlc.arg(tag_id)::text
    UNION ALL
    SELECT 1 FROM directory.tag_slug_aliases WHERE slug = sqlc.arg(slug)::text AND tag_id::text <> sqlc.arg(tag_id)::text
)::boolean;

-- name: ExistingTagSlug :one
SELECT t.slug FROM directory.tags t JOIN directory.tag_labels l ON l.tag_id=t.id WHERE l.normalized_name=$1 AND l.is_enabled AND t.is_enabled;

-- name: SlugCandidateConflicts :many
SELECT t.id,t.name,t.slug FROM directory.tag_dictionary t WHERE t.id::text<>sqlc.arg(tag_id)::text AND (t.slug=sqlc.arg(slug)::text OR EXISTS(SELECT 1 FROM directory.tag_slug_aliases a WHERE a.tag_id=t.id AND a.slug=sqlc.arg(slug)::text));

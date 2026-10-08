-- name: LockTaxonomy :exec
SELECT pg_advisory_xact_lock(74821953);

-- name: ListManagedTags :many
SELECT * FROM directory.tag_dictionary ORDER BY id;

-- name: ListManagedCascades :many
SELECT * FROM directory.tag_cascades ORDER BY id;

-- name: GetCanonicalTag :one
SELECT t.* FROM directory.tag_dictionary t
WHERE t.id=$1 OR t.id=(SELECT a.tag_id FROM directory.tag_identity_aliases a WHERE a.alias_id=$1);

-- name: GetCanonicalCascade :one
WITH RECURSIVE chain AS (
 SELECT t.* FROM directory.tag_cascades t WHERE t.id=$1
 UNION ALL SELECT c.* FROM directory.tag_cascades c JOIN chain prior ON c.id=prior.merged_into_id
) SELECT * FROM chain WHERE merged_into_id IS NULL;

-- name: TaxonomySlugOwner :many
SELECT t.id FROM directory.tags t WHERE t.slug=$1
UNION SELECT a.tag_id AS id FROM directory.tag_slug_aliases a WHERE a.slug=$1;

-- name: ReserveTaxonomySlug :exec
INSERT INTO directory.tag_slug_aliases(slug,tag_id) VALUES($1,$2)
ON CONFLICT(slug) DO UPDATE SET tag_id=EXCLUDED.tag_id;

-- name: CreateManagedTag :one
INSERT INTO directory.tag_dictionary(name,normalized_name,slug,description)
VALUES(sqlc.arg(name),lower(btrim(sqlc.arg(name))),sqlc.arg(slug),sqlc.arg(description)) RETURNING *;

-- name: UpdateManagedTag :exec
UPDATE directory.tag_dictionary SET name=$2,normalized_name=lower(btrim($2)),slug=$3,description=$4,is_enabled=$5 WHERE id=$1;

-- name: DeleteManagedTag :exec
DELETE FROM directory.tags WHERE id=$1;

-- name: ReadableSiteCascades :many
SELECT c.*, p.name AS level1_name,p.slug AS level1_slug,p.description AS level1_description,
 s.name AS level2_name,s.slug AS level2_slug,s.description AS level2_description
FROM directory.tag_cascades c JOIN directory.tag_dictionary p ON p.id=c.level1_tag_id JOIN directory.tag_dictionary s ON s.id=c.level2_tag_id
WHERE c.scope='SITE' AND c.merged_into_id IS NULL ORDER BY c.sort_order,c.id;

-- name: EnableCanonicalTag :exec
UPDATE directory.tags SET is_enabled=true WHERE id=$1;

-- name: ListTagIdentityAliases :many
SELECT * FROM directory.tag_identity_aliases ORDER BY alias_id;

-- name: ListTagLabels :many
SELECT * FROM directory.tag_labels ORDER BY tag_id,created_at,id;

-- name: GetTagLabel :one
SELECT * FROM directory.tag_labels WHERE id=$1;

-- name: GetTagLabelByNormalizedName :one
SELECT * FROM directory.tag_labels WHERE normalized_name=$1;

-- name: ListEnabledTagLabels :many
SELECT l.*,t.slug FROM directory.tag_labels l JOIN directory.tags t ON t.id=l.tag_id WHERE l.is_enabled AND t.is_enabled ORDER BY l.name,l.id;

-- name: SetSiteClassificationLabels :one
UPDATE directory.sites SET primary_label_id=$2,secondary_label_id=$3 WHERE id=$1 RETURNING revision;

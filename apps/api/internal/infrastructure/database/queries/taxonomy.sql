-- name: LockTaxonomy :exec
SELECT pg_advisory_xact_lock(74821953);

-- name: ListManagedTags :many
SELECT * FROM directory.tags ORDER BY id;

-- name: ListManagedCascades :many
SELECT * FROM directory.tag_cascades ORDER BY id;

-- name: GetCanonicalTag :one
SELECT t.* FROM directory.tags t
WHERE t.id=$1 OR t.id=(SELECT a.tag_id FROM directory.tag_identity_aliases a WHERE a.alias_kind='TAG' AND a.alias_id=$1);

-- name: GetCanonicalCascade :one
WITH RECURSIVE chain AS (
 SELECT t.* FROM directory.tag_cascades t WHERE t.id=$1
 UNION ALL SELECT c.* FROM directory.tag_cascades c JOIN chain prior ON c.id=prior.merged_into_id
) SELECT * FROM chain WHERE merged_into_id IS NULL;

-- name: CreateManagedTag :one
INSERT INTO directory.tags(name,description)
VALUES(sqlc.arg(name),sqlc.arg(description)) RETURNING *;

-- name: UpdateManagedTag :exec
UPDATE directory.tags SET name=$2,description=$3,is_enabled=$4 WHERE id=$1;

-- name: DeleteManagedTag :exec
DELETE FROM directory.tags WHERE id=$1;

-- name: ReadableSiteCascades :many
SELECT c.*, p.name AS level1_name,p.description AS level1_description,
 s.name AS level2_name,s.description AS level2_description
FROM directory.tag_cascades c JOIN directory.tags p ON p.id=c.level1_tag_id JOIN directory.tags s ON s.id=c.level2_tag_id
WHERE c.scope='SITE' AND c.merged_into_id IS NULL ORDER BY c.sort_order,c.id;

-- name: EnableCanonicalTag :exec
UPDATE directory.tags SET is_enabled=true WHERE id=$1;

-- name: ListTagIdentityAliases :many
SELECT * FROM directory.tag_identity_aliases ORDER BY alias_kind,alias_id;

-- name: GetHistoricalTagLabel :one
SELECT t.* FROM directory.tags t JOIN directory.tag_identity_aliases a ON a.tag_id=t.id
WHERE a.alias_kind='LABEL' AND a.alias_id=$1;

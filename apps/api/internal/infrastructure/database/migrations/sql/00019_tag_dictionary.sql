-- +goose Up
SELECT pg_advisory_xact_lock(74821953);
DROP TRIGGER tags_check_structure ON directory.tags;
DROP TRIGGER cascades_check_structure ON directory.tag_cascades;
DROP FUNCTION directory.check_taxonomy_structure();
ALTER TABLE directory.tag_cascades DROP CONSTRAINT tag_cascades_tags_check;
DROP INDEX directory.tag_cascades_canonical_pair_idx;
DROP INDEX directory.tags_level_name_unique_idx;

CREATE TABLE directory.tag_identity_aliases (
    alias_id uuid PRIMARY KEY, -- Historical tag identifier, including retained seed identifiers.
    tag_id uuid NOT NULL REFERENCES directory.tags(id) ON DELETE RESTRICT, -- Current dictionary owner.
    system_key text UNIQUE, -- Historical stable import identity.
    snapshot jsonb NOT NULL, -- Original dictionary row before canonicalization.
    archived_at timestamptz NOT NULL DEFAULT now() -- Identity archival time.
);
CREATE TABLE directory.tag_assignment_archive (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- Unique archive event identifier.
    scope text NOT NULL CHECK (scope IN ('SITE','ARTICLE')), -- Assignment object family.
    object_id uuid NOT NULL, -- Historical object identifier.
    snapshot jsonb NOT NULL, -- Original complete association before rewriting.
    reason text NOT NULL, -- Operation that rewrote the association.
    archived_at timestamptz NOT NULL DEFAULT now() -- Archive event time.
);

CREATE UNLOGGED TABLE directory.migration_tag_dictionary_map AS
WITH RECURSIVE chain AS (
 SELECT id AS original_id,id,merged_into_id,ARRAY[id] AS visited FROM directory.tags
 UNION ALL
 SELECT c.original_id,t.id,t.merged_into_id,c.visited||t.id FROM chain c
 JOIN directory.tags t ON t.id=c.merged_into_id WHERE NOT t.id=ANY(c.visited)
), roots AS (
 SELECT original_id,id FROM chain WHERE merged_into_id IS NULL
), winners AS (
 SELECT id,first_value(id) OVER (PARTITION BY normalized_name ORDER BY
   CASE WHEN system_key='other' THEN 0 WHEN system_key IS NOT NULL AND taxonomy_level=1 THEN 1
        WHEN system_key IS NOT NULL THEN 2 WHEN slug NOT LIKE 'legacy-%' THEN 3 ELSE 4 END,
   created_at,id) AS canonical_id FROM directory.tags WHERE merged_into_id IS NULL
) SELECT roots.original_id AS old_id,winners.canonical_id AS tag_id FROM roots JOIN winners ON winners.id=roots.id;

-- +goose StatementBegin
DO $$ BEGIN
 IF (SELECT count(*) FROM directory.migration_tag_dictionary_map) <> (SELECT count(*) FROM directory.tags) THEN
   RAISE EXCEPTION 'tag dictionary migration blocked: unresolved historical identity';
 END IF;
 IF EXISTS (
   SELECT a.site_id,m.tag_id FROM directory.site_tags a JOIN directory.migration_tag_dictionary_map m ON m.old_id=a.tag_id
   GROUP BY a.site_id,m.tag_id HAVING count(DISTINCT jsonb_build_array(a.role,a.assignment_source,a.note))>1
 ) OR EXISTS (
   SELECT a.article_id,m.tag_id FROM content.article_tags a JOIN directory.migration_tag_dictionary_map m ON m.old_id=a.tag_id
   GROUP BY a.article_id,m.tag_id HAVING count(DISTINCT jsonb_build_array(a.role,a.assignment_source,a.note))>1
 ) THEN RAISE EXCEPTION 'tag dictionary migration blocked: conflicting assignment role, source or note'; END IF;
END $$;
-- +goose StatementEnd

INSERT INTO directory.tag_identity_aliases(alias_id,tag_id,system_key,snapshot)
SELECT t.id,m.tag_id,t.system_key,to_jsonb(t) FROM directory.tags t JOIN directory.migration_tag_dictionary_map m ON m.old_id=t.id;
INSERT INTO directory.tag_assignment_archive(scope,object_id,snapshot,reason)
SELECT 'SITE',site_id,to_jsonb(a),'dictionary_migration' FROM directory.site_tags a
UNION ALL SELECT 'ARTICLE',article_id,to_jsonb(a),'dictionary_migration' FROM content.article_tags a;
CREATE UNLOGGED TABLE directory.migration_dictionary_site_assignments AS SELECT * FROM directory.site_tags;
CREATE UNLOGGED TABLE directory.migration_dictionary_article_assignments AS SELECT * FROM content.article_tags;
DELETE FROM directory.site_tags;
DELETE FROM content.article_tags;

UPDATE directory.tag_slug_aliases a SET tag_id=m.tag_id FROM directory.migration_tag_dictionary_map m WHERE m.old_id=a.tag_id;
INSERT INTO directory.tag_slug_aliases(slug,tag_id)
SELECT t.slug,m.tag_id FROM directory.tags t JOIN directory.migration_tag_dictionary_map m ON m.old_id=t.id WHERE t.id<>m.tag_id
ON CONFLICT(slug) DO UPDATE SET tag_id=EXCLUDED.tag_id;
UPDATE directory.tags t SET is_enabled=g.enabled FROM (
 SELECT m.tag_id,bool_or(t.is_enabled) AS enabled FROM directory.tags t JOIN directory.migration_tag_dictionary_map m ON m.old_id=t.id GROUP BY m.tag_id
) g WHERE t.id=g.tag_id;
UPDATE directory.tag_cascades c SET level1_tag_id=p.tag_id,level2_tag_id=s.tag_id
FROM directory.migration_tag_dictionary_map p,directory.migration_tag_dictionary_map s WHERE p.old_id=c.level1_tag_id AND s.old_id=c.level2_tag_id;
CREATE UNLOGGED TABLE directory.migration_dictionary_path_map AS
SELECT id,first_value(id) OVER(PARTITION BY scope,level1_tag_id,level2_tag_id ORDER BY
 CASE WHEN taxonomy_key='other/topic-other-other' THEN 0 ELSE 1 END,created_at,id) AS canonical_id
FROM directory.tag_cascades WHERE merged_into_id IS NULL;
UPDATE directory.tag_cascades c SET is_enabled=g.enabled FROM (
 SELECT m.canonical_id,bool_or(c.is_enabled) AS enabled FROM directory.migration_dictionary_path_map m
 JOIN directory.tag_cascades c ON c.id=m.id GROUP BY m.canonical_id
) g WHERE c.id=g.canonical_id;
UPDATE directory.tag_cascades c SET merged_into_id=m.canonical_id,is_enabled=false
FROM directory.migration_dictionary_path_map m WHERE c.id=m.id AND m.id<>m.canonical_id;
UPDATE directory.tag_cascades c SET merged_into_id=m.canonical_id
FROM directory.migration_dictionary_path_map m WHERE c.merged_into_id=m.id AND m.id<>m.canonical_id;
UPDATE directory.sites s SET tag_cascade_id=m.canonical_id,revision=revision+1
FROM directory.migration_dictionary_path_map m WHERE s.tag_cascade_id=m.id AND m.id<>m.canonical_id;
UPDATE content.articles a SET tag_cascade_id=m.canonical_id
FROM directory.migration_dictionary_path_map m WHERE a.tag_cascade_id=m.id AND m.id<>m.canonical_id;

DROP TRIGGER tags_prevent_merge_cycle ON directory.tags;
DROP FUNCTION directory.prevent_tag_merge_cycle();
ALTER TABLE directory.tags DROP COLUMN merged_by, DROP COLUMN parent_id, DROP COLUMN taxonomy_level, DROP COLUMN is_fixed,
 DROP COLUMN system_key, DROP COLUMN merged_into_id, DROP COLUMN merged_at;
DELETE FROM directory.tags t USING directory.migration_tag_dictionary_map m WHERE t.id=m.old_id AND m.old_id<>m.tag_id;
CREATE UNIQUE INDEX tags_normalized_name_unique_idx ON directory.tags(normalized_name);
CREATE UNIQUE INDEX tag_cascades_canonical_pair_idx ON directory.tag_cascades(scope,level1_tag_id,level2_tag_id) WHERE merged_into_id IS NULL;

INSERT INTO directory.site_tags(site_id,tag_id,role,assignment_source,position,note,created_at)
SELECT site_id,tag_id,role,assignment_source,CASE WHEN role='TERTIARY' THEN row_number() OVER(PARTITION BY site_id,role ORDER BY position,tag_id)::smallint END,note,created_at
FROM (SELECT DISTINCT ON(a.site_id,m.tag_id) a.site_id,m.tag_id,a.role,a.assignment_source,a.position,a.note,a.created_at
 FROM directory.migration_dictionary_site_assignments a JOIN directory.migration_tag_dictionary_map m ON m.old_id=a.tag_id
 JOIN directory.sites s ON s.id=a.site_id JOIN directory.tag_cascades c ON c.id=s.tag_cascade_id
 WHERE a.role<>'TERTIARY' OR m.tag_id NOT IN(c.level1_tag_id,c.level2_tag_id)
 ORDER BY a.site_id,m.tag_id,a.position,a.created_at) assignments;
INSERT INTO content.article_tags(article_id,tag_id,role,assignment_source,position,note,created_at)
SELECT article_id,tag_id,role,assignment_source,CASE WHEN role='TERTIARY' THEN row_number() OVER(PARTITION BY article_id,role ORDER BY position,tag_id)::smallint END,note,created_at
FROM (SELECT DISTINCT ON(a.article_id,m.tag_id) a.article_id,m.tag_id,a.role,a.assignment_source,a.position,a.note,a.created_at
 FROM directory.migration_dictionary_article_assignments a JOIN directory.migration_tag_dictionary_map m ON m.old_id=a.tag_id
 JOIN content.articles s ON s.id=a.article_id JOIN directory.tag_cascades c ON c.id=s.tag_cascade_id
 WHERE a.role<>'TERTIARY' OR m.tag_id NOT IN(c.level1_tag_id,c.level2_tag_id)
 ORDER BY a.article_id,m.tag_id,a.position,a.created_at) assignments;

DROP TABLE directory.migration_tag_dictionary_map;
DROP TABLE directory.migration_dictionary_site_assignments;
DROP TABLE directory.migration_dictionary_article_assignments;
DROP TABLE directory.migration_dictionary_path_map;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.canonical_tag_slug(value text) RETURNS text LANGUAGE sql STABLE AS $$
 SELECT COALESCE((SELECT t.slug FROM directory.tags t WHERE t.slug=value
 UNION SELECT t.slug FROM directory.tag_slug_aliases a JOIN directory.tags t ON t.id=a.tag_id WHERE a.slug=value LIMIT 1),value);
$$;
-- +goose StatementEnd
COMMENT ON TABLE directory.tags IS 'Globally unique tag dictionary; classification roles belong to associations.';
COMMENT ON TABLE directory.tag_identity_aliases IS 'Historical tag IDs and import keys resolving directly to current dictionary identities.';
COMMENT ON COLUMN directory.tag_identity_aliases.alias_id IS 'Historical tag identifier, including retained seed identifiers.'; -- Historical tag identifier, including retained seed identifiers.
COMMENT ON COLUMN directory.tag_identity_aliases.tag_id IS 'Current dictionary owner.'; -- Current dictionary owner.
COMMENT ON COLUMN directory.tag_identity_aliases.system_key IS 'Historical stable import identity.'; -- Historical stable import identity.
COMMENT ON COLUMN directory.tag_identity_aliases.snapshot IS 'Original dictionary row before canonicalization.'; -- Original dictionary row before canonicalization.
COMMENT ON COLUMN directory.tag_identity_aliases.archived_at IS 'Identity archival time.'; -- Identity archival time.
COMMENT ON TABLE directory.tag_assignment_archive IS 'Lossless original associations retained before dictionary rewrites.';
COMMENT ON COLUMN directory.tag_assignment_archive.id IS 'Unique archive event identifier.'; -- Unique archive event identifier.
COMMENT ON COLUMN directory.tag_assignment_archive.scope IS 'Assignment object family.'; -- Assignment object family.
COMMENT ON COLUMN directory.tag_assignment_archive.object_id IS 'Historical object identifier.'; -- Historical object identifier.
COMMENT ON COLUMN directory.tag_assignment_archive.snapshot IS 'Original complete association before rewriting.'; -- Original complete association before rewriting.
COMMENT ON COLUMN directory.tag_assignment_archive.reason IS 'Operation that rewrote the association.'; -- Operation that rewrote the association.
COMMENT ON COLUMN directory.tag_assignment_archive.archived_at IS 'Archive event time.'; -- Archive event time.
COMMENT ON TABLE directory.tag_cascades IS 'Scoped classification paths; the same dictionary tag may occupy both roles.';
COMMENT ON COLUMN directory.tag_cascades.level1_tag_id IS 'Tag occupying the primary classification role.'; -- Tag occupying the primary classification role.
COMMENT ON COLUMN directory.tag_cascades.level2_tag_id IS 'Tag occupying the secondary classification role.'; -- Tag occupying the secondary classification role.
GRANT SELECT,INSERT,UPDATE,DELETE ON directory.tag_identity_aliases TO api_runtime;
GRANT SELECT,INSERT ON directory.tag_assignment_archive TO api_runtime;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 RAISE EXCEPTION 'tag dictionary consolidation is irreversible; restore the pre-upgrade database backup';
END $$;
-- +goose StatementEnd

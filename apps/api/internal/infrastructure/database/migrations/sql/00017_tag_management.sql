-- +goose Up
ALTER TABLE directory.tags
    ADD COLUMN taxonomy_level smallint NOT NULL DEFAULT 3 CHECK (taxonomy_level BETWEEN 1 AND 3), -- Intrinsic dictionary layer, independent of assignment role.
    ADD COLUMN parent_id uuid REFERENCES directory.tags(id) ON DELETE RESTRICT; -- Primary parent of an intrinsic secondary tag.
UPDATE directory.tags t SET taxonomy_level=1
WHERE EXISTS (SELECT 1 FROM directory.tag_cascades c WHERE c.level1_tag_id=t.id);
UPDATE directory.tags t SET taxonomy_level=2, parent_id=c.level1_tag_id
FROM directory.tag_cascades c WHERE c.level2_tag_id=t.id AND c.scope='SITE';
ALTER TABLE directory.tags ADD CONSTRAINT tags_parent_level_check CHECK
    ((taxonomy_level=2 AND parent_id IS NOT NULL AND parent_id<>id) OR (taxonomy_level<>2 AND parent_id IS NULL));
DROP INDEX directory.tags_flexible_normalized_name_unique_idx;
CREATE UNIQUE INDEX tags_level_name_unique_idx ON directory.tags
    (taxonomy_level, COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid), normalized_name)
    WHERE merged_into_id IS NULL;
ALTER TABLE directory.tag_cascades
    ADD COLUMN merged_into_id uuid REFERENCES directory.tag_cascades(id) ON DELETE RESTRICT, -- Replacement path retained for historical audit references.
    ADD CONSTRAINT cascades_merge_self_check CHECK (merged_into_id IS NULL OR merged_into_id<>id);
ALTER TABLE directory.tag_cascades DROP CONSTRAINT tag_cascades_scope_pair_unique;
CREATE UNIQUE INDEX tag_cascades_canonical_pair_idx ON directory.tag_cascades(scope,level1_tag_id,level2_tag_id) WHERE merged_into_id IS NULL;
CREATE TABLE directory.tag_slug_aliases (
    slug text PRIMARY KEY, -- Reserved historical slug.
    tag_id uuid NOT NULL REFERENCES directory.tags(id) ON DELETE RESTRICT, -- Canonical owner of the historical slug.
    CONSTRAINT tag_slug_aliases_format_check CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);
COMMENT ON COLUMN directory.tag_slug_aliases.slug IS 'Reserved historical slug.';
COMMENT ON COLUMN directory.tag_slug_aliases.tag_id IS 'Canonical owner of the historical slug.';
COMMENT ON TABLE directory.tag_slug_aliases IS 'Reserved historical slug namespace pointing to current tags.';
COMMENT ON COLUMN directory.tags.taxonomy_level IS 'Intrinsic dictionary layer, independent of assignment role.';
COMMENT ON COLUMN directory.tags.parent_id IS 'Primary parent of an intrinsic secondary tag.';
COMMENT ON COLUMN directory.tag_cascades.merged_into_id IS 'Replacement path retained for historical audit references.';
GRANT SELECT, INSERT, UPDATE, DELETE ON directory.tag_slug_aliases, directory.tag_cascades TO api_runtime;
GRANT DELETE ON directory.tags TO api_runtime;

-- Deferred checks permit atomic graph rewrites but reject invalid final structure.
-- +goose StatementBegin
CREATE FUNCTION directory.check_taxonomy_structure() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM directory.tags child LEFT JOIN directory.tags parent ON parent.id=child.parent_id
        WHERE child.merged_into_id IS NULL AND child.taxonomy_level=2
          AND (parent.taxonomy_level<>1 OR parent.merged_into_id IS NOT NULL)) THEN
        RAISE EXCEPTION 'secondary taxonomy parent must be canonical primary';
    END IF;
    IF EXISTS (SELECT 1 FROM directory.tag_cascades c
        JOIN directory.tags p ON p.id=c.level1_tag_id JOIN directory.tags s ON s.id=c.level2_tag_id
        WHERE c.merged_into_id IS NULL AND (p.taxonomy_level<>1 OR s.taxonomy_level<>2 OR s.parent_id<>p.id
          OR p.merged_into_id IS NOT NULL OR s.merged_into_id IS NOT NULL)) THEN
        RAISE EXCEPTION 'taxonomy cascade must match canonical parent and child';
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER tags_check_structure AFTER INSERT OR UPDATE OR DELETE ON directory.tags
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION directory.check_taxonomy_structure();
CREATE CONSTRAINT TRIGGER cascades_check_structure AFTER INSERT OR UPDATE OR DELETE ON directory.tag_cascades
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION directory.check_taxonomy_structure();

-- Existing disabled assignments may be saved unchanged; fresh selection requires an enabled path.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.ensure_site_cascade_scope() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.tag_cascade_id IS NULL THEN
        SELECT id INTO NEW.tag_cascade_id FROM directory.tag_cascades WHERE scope='SITE' AND taxonomy_key='other/topic-other-other';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM directory.tag_cascades WHERE id=NEW.tag_cascade_id AND scope='SITE'
        AND merged_into_id IS NULL AND (is_enabled OR (TG_OP='UPDATE' AND OLD.tag_cascade_id=NEW.tag_cascade_id))) THEN
        RAISE EXCEPTION 'site tag cascade must reference an available SITE path';
    END IF;
    IF EXISTS (SELECT 1 FROM directory.site_tags a JOIN directory.tag_cascades c ON c.id=NEW.tag_cascade_id
        WHERE a.site_id=NEW.id AND a.role='TERTIARY' AND a.tag_id IN (c.level1_tag_id,c.level2_tag_id)) THEN
        RAISE EXCEPTION 'site tertiary tags must not repeat classification';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION content.ensure_article_cascade_scope() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM directory.tag_cascades WHERE id=NEW.tag_cascade_id AND scope='ARTICLE'
        AND merged_into_id IS NULL AND (is_enabled OR (TG_OP='UPDATE' AND OLD.tag_cascade_id=NEW.tag_cascade_id))) THEN
        RAISE EXCEPTION 'article tag cascade must reference an available ARTICLE path';
    END IF;
    IF EXISTS (SELECT 1 FROM content.article_tags a JOIN directory.tag_cascades c ON c.id=NEW.tag_cascade_id
        WHERE a.article_id=NEW.id AND a.role='TERTIARY' AND a.tag_id IN (c.level1_tag_id,c.level2_tag_id)) THEN
        RAISE EXCEPTION 'article tertiary tags must not repeat classification';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.canonical_tag_slug(value text) RETURNS text LANGUAGE sql STABLE AS $$
 WITH RECURSIVE chain AS (
   SELECT t.id,t.slug,t.merged_into_id FROM directory.tags t WHERE t.slug=value
   UNION
   SELECT t.id,t.slug,t.merged_into_id FROM directory.tag_slug_aliases a JOIN directory.tags t ON t.id=a.tag_id WHERE a.slug=value
   UNION ALL
   SELECT t.id,t.slug,t.merged_into_id FROM directory.tags t JOIN chain c ON t.id=c.merged_into_id
 ) SELECT COALESCE((SELECT slug FROM chain WHERE merged_into_id IS NULL LIMIT 1),value);
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION directory.canonical_tag_slugs(values_ text[]) RETURNS text[] LANGUAGE sql STABLE AS $$
 SELECT COALESCE(array_agg(DISTINCT directory.canonical_tag_slug(value)),'{}'::text[]) FROM unnest(values_) AS value;
$$;
-- +goose StatementEnd

COMMENT ON FUNCTION directory.check_taxonomy_structure() IS 'Validates canonical parent and cascade structure at transaction commit.';
COMMENT ON FUNCTION directory.canonical_tag_slug(text) IS 'Resolves historical slugs and merged tag identities to the current canonical slug.';
COMMENT ON FUNCTION directory.canonical_tag_slugs(text[]) IS 'Resolves and deduplicates a set of historical and current tag slugs.';
COMMENT ON TRIGGER tags_check_structure ON directory.tags IS 'Checks dictionary hierarchy after atomic taxonomy changes.';
COMMENT ON TRIGGER cascades_check_structure ON directory.tag_cascades IS 'Checks canonical paths after atomic taxonomy changes.';

-- +goose Down
DROP FUNCTION directory.canonical_tag_slugs(text[]);
DROP FUNCTION directory.canonical_tag_slug(text);
DROP TRIGGER tags_check_structure ON directory.tags;
DROP TRIGGER cascades_check_structure ON directory.tag_cascades;
DROP FUNCTION directory.check_taxonomy_structure();
DROP TABLE directory.tag_slug_aliases;
DROP INDEX directory.tags_level_name_unique_idx;
ALTER TABLE directory.tags DROP CONSTRAINT tags_parent_level_check, DROP COLUMN parent_id, DROP COLUMN taxonomy_level;
DROP INDEX directory.tag_cascades_canonical_pair_idx;
ALTER TABLE directory.tag_cascades DROP COLUMN merged_into_id;
ALTER TABLE directory.tag_cascades ADD CONSTRAINT tag_cascades_scope_pair_unique UNIQUE(scope,level1_tag_id,level2_tag_id);
CREATE UNIQUE INDEX tags_flexible_normalized_name_unique_idx ON directory.tags(normalized_name) WHERE NOT is_fixed AND merged_into_id IS NULL;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.ensure_site_cascade_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.tag_cascade_id IS NULL THEN
        SELECT id INTO NEW.tag_cascade_id
          FROM directory.tag_cascades
         WHERE scope = 'SITE' AND taxonomy_key = 'other/topic-other-other';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM directory.tag_cascades
         WHERE id = NEW.tag_cascade_id AND scope = 'SITE' AND is_enabled
    ) THEN
        RAISE EXCEPTION 'site tag cascade must reference an enabled SITE path';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM directory.site_tags AS assignment
          JOIN directory.tag_cascades AS cascade ON cascade.id = NEW.tag_cascade_id
         WHERE assignment.site_id = NEW.id
           AND assignment.role = 'TERTIARY'
           AND assignment.tag_id IN (cascade.level1_tag_id, cascade.level2_tag_id)
    ) THEN
        RAISE EXCEPTION 'site tertiary tags must not repeat its fixed classification';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION content.ensure_article_cascade_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM directory.tag_cascades
         WHERE id = NEW.tag_cascade_id AND scope = 'ARTICLE' AND is_enabled
    ) THEN
        RAISE EXCEPTION 'article tag cascade must reference an enabled ARTICLE path';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM content.article_tags AS assignment
          JOIN directory.tag_cascades AS cascade ON cascade.id = NEW.tag_cascade_id
         WHERE assignment.article_id = NEW.id
           AND assignment.role = 'TERTIARY'
           AND assignment.tag_id IN (cascade.level1_tag_id, cascade.level2_tag_id)
    ) THEN
        RAISE EXCEPTION 'article tertiary tags must not repeat its fixed classification';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

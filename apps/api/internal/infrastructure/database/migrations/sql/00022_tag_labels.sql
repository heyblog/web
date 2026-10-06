-- +goose Up
SELECT pg_advisory_xact_lock(74821953);
CREATE TABLE directory.tag_labels (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    tag_id uuid NOT NULL REFERENCES directory.tags(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    normalized_name text NOT NULL GENERATED ALWAYS AS (lower(btrim(name))) STORED UNIQUE,
    is_enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE directory.tags ADD COLUMN default_label_id uuid;
INSERT INTO directory.tag_labels(id,tag_id,name,created_at,updated_at) SELECT id,id,name,created_at,updated_at FROM directory.tags;
-- Backfilling representation must not change edit timestamps or optimistic revisions.
ALTER TABLE directory.tags DISABLE TRIGGER tags_touch_updated_at;
UPDATE directory.tags SET default_label_id=id;
SET CONSTRAINTS ALL IMMEDIATE;
ALTER TABLE directory.tags ENABLE TRIGGER tags_touch_updated_at;
ALTER TABLE directory.tags ALTER COLUMN default_label_id SET NOT NULL;
ALTER TABLE directory.tags ADD CONSTRAINT tags_default_label_fk FOREIGN KEY(default_label_id) REFERENCES directory.tag_labels(id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE directory.site_tags ADD COLUMN label_id uuid REFERENCES directory.tag_labels(id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE content.article_tags ADD COLUMN label_id uuid REFERENCES directory.tag_labels(id) DEFERRABLE INITIALLY DEFERRED;
UPDATE directory.site_tags SET label_id=tag_id;
UPDATE content.article_tags SET label_id=tag_id;
SET CONSTRAINTS ALL IMMEDIATE;
ALTER TABLE directory.site_tags ALTER COLUMN label_id SET NOT NULL;
ALTER TABLE content.article_tags ALTER COLUMN label_id SET NOT NULL;
ALTER TABLE directory.sites ADD COLUMN primary_label_id uuid REFERENCES directory.tag_labels(id) DEFERRABLE INITIALLY DEFERRED, ADD COLUMN secondary_label_id uuid REFERENCES directory.tag_labels(id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE content.articles ADD COLUMN primary_label_id uuid REFERENCES directory.tag_labels(id) DEFERRABLE INITIALLY DEFERRED, ADD COLUMN secondary_label_id uuid REFERENCES directory.tag_labels(id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE directory.sites DISABLE TRIGGER sites_touch_site;
ALTER TABLE content.articles DISABLE TRIGGER articles_touch_updated_at;
UPDATE directory.sites s SET primary_label_id=c.level1_tag_id,secondary_label_id=c.level2_tag_id FROM directory.tag_cascades c WHERE c.id=s.tag_cascade_id;
UPDATE content.articles s SET primary_label_id=c.level1_tag_id,secondary_label_id=c.level2_tag_id FROM directory.tag_cascades c WHERE c.id=s.tag_cascade_id;
SET CONSTRAINTS ALL IMMEDIATE;
ALTER TABLE directory.sites ENABLE TRIGGER sites_touch_site;
ALTER TABLE content.articles ENABLE TRIGGER articles_touch_updated_at;
ALTER TABLE directory.sites ALTER COLUMN primary_label_id SET NOT NULL, ALTER COLUMN secondary_label_id SET NOT NULL;
ALTER TABLE content.articles ALTER COLUMN primary_label_id SET NOT NULL, ALTER COLUMN secondary_label_id SET NOT NULL;
SET CONSTRAINTS ALL IMMEDIATE;
CREATE INDEX tag_labels_tag_idx ON directory.tag_labels(tag_id);
CREATE INDEX sites_primary_label_idx ON directory.sites(primary_label_id);
CREATE INDEX sites_secondary_label_idx ON directory.sites(secondary_label_id);
CREATE INDEX articles_primary_label_idx ON content.articles(primary_label_id);
CREATE INDEX articles_secondary_label_idx ON content.articles(secondary_label_id);
CREATE INDEX site_tags_label_idx ON directory.site_tags(label_id);
CREATE INDEX article_tags_label_idx ON content.article_tags(label_id);
ALTER TABLE directory.tags DROP COLUMN normalized_name, DROP COLUMN name;
CREATE VIEW directory.tag_dictionary AS SELECT t.id,l.name,l.normalized_name,t.slug,t.description,t.is_enabled,t.created_at,t.updated_at,t.default_label_id FROM directory.tags t JOIN directory.tag_labels l ON l.id=t.default_label_id;
-- +goose StatementBegin
CREATE FUNCTION directory.write_tag_dictionary() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  NEW.id:=COALESCE(NEW.id,uuidv7()); NEW.default_label_id:=uuidv7();
  INSERT INTO directory.tags(id,slug,description,is_enabled,default_label_id) VALUES(NEW.id,NEW.slug,COALESCE(NEW.description,''),COALESCE(NEW.is_enabled,true),NEW.default_label_id);
  INSERT INTO directory.tag_labels(id,tag_id,name) VALUES(NEW.default_label_id,NEW.id,btrim(NEW.name));
 ELSE
  UPDATE directory.tags SET slug=NEW.slug,description=NEW.description,is_enabled=NEW.is_enabled WHERE id=OLD.id;
  UPDATE directory.tag_labels SET name=btrim(NEW.name),updated_at=now() WHERE id=OLD.default_label_id;
 END IF;
 SELECT * INTO NEW FROM directory.tag_dictionary WHERE id=NEW.id;
 RETURN NEW;
END $$;
CREATE TRIGGER tag_dictionary_write INSTEAD OF INSERT OR UPDATE ON directory.tag_dictionary FOR EACH ROW EXECUTE FUNCTION directory.write_tag_dictionary();
CREATE FUNCTION directory.fill_selected_tag_labels() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE c directory.tag_cascades;
BEGIN
 IF TG_TABLE_NAME IN ('sites','articles') THEN
  SELECT * INTO c FROM directory.tag_cascades WHERE id=NEW.tag_cascade_id;
  IF NEW.primary_label_id IS NULL THEN SELECT default_label_id INTO NEW.primary_label_id FROM directory.tags WHERE id=c.level1_tag_id; END IF;
  IF NEW.secondary_label_id IS NULL THEN SELECT default_label_id INTO NEW.secondary_label_id FROM directory.tags WHERE id=c.level2_tag_id; END IF;
 ELSE
  IF NEW.label_id IS NULL THEN SELECT default_label_id INTO NEW.label_id FROM directory.tags WHERE id=NEW.tag_id; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE FUNCTION directory.check_tag_label_ownership() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE invalid boolean:=false; ref uuid;
BEGIN
 IF TG_TABLE_NAME='tags' THEN
  SELECT EXISTS(SELECT 1 FROM directory.tags t LEFT JOIN directory.tag_labels l ON l.id=t.default_label_id WHERE t.id=NEW.id AND (l.tag_id IS DISTINCT FROM t.id OR NOT l.is_enabled)) INTO invalid;
 ELSIF TG_TABLE_NAME='tag_labels' THEN
  ref:=COALESCE(NEW.id,OLD.id);
  SELECT EXISTS(SELECT 1 FROM directory.tags t JOIN directory.tag_labels l ON l.id=t.default_label_id WHERE l.id=ref AND (l.tag_id<>t.id OR NOT l.is_enabled))
  OR EXISTS(SELECT 1 FROM directory.site_tags a JOIN directory.tag_labels l ON l.id=a.label_id WHERE l.id=ref AND l.tag_id<>a.tag_id)
  OR EXISTS(SELECT 1 FROM content.article_tags a JOIN directory.tag_labels l ON l.id=a.label_id WHERE l.id=ref AND l.tag_id<>a.tag_id)
  OR EXISTS(SELECT 1 FROM directory.sites s JOIN directory.tag_cascades c ON c.id=s.tag_cascade_id JOIN directory.tag_labels l ON l.id=ref WHERE (s.primary_label_id=ref AND l.tag_id<>c.level1_tag_id) OR (s.secondary_label_id=ref AND l.tag_id<>c.level2_tag_id))
  OR EXISTS(SELECT 1 FROM content.articles s JOIN directory.tag_cascades c ON c.id=s.tag_cascade_id JOIN directory.tag_labels l ON l.id=ref WHERE (s.primary_label_id=ref AND l.tag_id<>c.level1_tag_id) OR (s.secondary_label_id=ref AND l.tag_id<>c.level2_tag_id)) INTO invalid;
 ELSIF TG_TABLE_NAME='site_tags' THEN
  SELECT EXISTS(SELECT 1 FROM directory.site_tags a JOIN directory.tag_labels l ON l.id=a.label_id WHERE a.site_id=NEW.site_id AND a.tag_id=NEW.tag_id AND l.tag_id<>a.tag_id) INTO invalid;
 ELSIF TG_TABLE_NAME='article_tags' THEN
  SELECT EXISTS(SELECT 1 FROM content.article_tags a JOIN directory.tag_labels l ON l.id=a.label_id WHERE a.article_id=NEW.article_id AND a.tag_id=NEW.tag_id AND l.tag_id<>a.tag_id) INTO invalid;
 ELSIF TG_TABLE_NAME='sites' THEN
  SELECT EXISTS(SELECT 1 FROM directory.sites s JOIN directory.tag_cascades c ON c.id=s.tag_cascade_id JOIN directory.tag_labels p ON p.id=s.primary_label_id JOIN directory.tag_labels q ON q.id=s.secondary_label_id WHERE s.id=NEW.id AND (p.tag_id<>c.level1_tag_id OR q.tag_id<>c.level2_tag_id)) INTO invalid;
 ELSIF TG_TABLE_NAME='articles' THEN
  SELECT EXISTS(SELECT 1 FROM content.articles s JOIN directory.tag_cascades c ON c.id=s.tag_cascade_id JOIN directory.tag_labels p ON p.id=s.primary_label_id JOIN directory.tag_labels q ON q.id=s.secondary_label_id WHERE s.id=NEW.id AND (p.tag_id<>c.level1_tag_id OR q.tag_id<>c.level2_tag_id)) INTO invalid;
 ELSIF TG_TABLE_NAME='tag_cascades' THEN
  SELECT EXISTS(SELECT 1 FROM directory.sites s JOIN directory.tag_cascades c ON c.id=s.tag_cascade_id JOIN directory.tag_labels p ON p.id=s.primary_label_id JOIN directory.tag_labels q ON q.id=s.secondary_label_id WHERE c.id=NEW.id AND (p.tag_id<>c.level1_tag_id OR q.tag_id<>c.level2_tag_id))
  OR EXISTS(SELECT 1 FROM content.articles s JOIN directory.tag_cascades c ON c.id=s.tag_cascade_id JOIN directory.tag_labels p ON p.id=s.primary_label_id JOIN directory.tag_labels q ON q.id=s.secondary_label_id WHERE c.id=NEW.id AND (p.tag_id<>c.level1_tag_id OR q.tag_id<>c.level2_tag_id)) INTO invalid;
 END IF;
 IF invalid THEN RAISE EXCEPTION 'tag label ownership mismatch' USING ERRCODE='23514'; END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER sites_fill_labels BEFORE INSERT OR UPDATE ON directory.sites FOR EACH ROW EXECUTE FUNCTION directory.fill_selected_tag_labels();
CREATE TRIGGER articles_fill_labels BEFORE INSERT OR UPDATE ON content.articles FOR EACH ROW EXECUTE FUNCTION directory.fill_selected_tag_labels();
CREATE TRIGGER site_tags_fill_labels BEFORE INSERT OR UPDATE ON directory.site_tags FOR EACH ROW EXECUTE FUNCTION directory.fill_selected_tag_labels();
CREATE TRIGGER article_tags_fill_labels BEFORE INSERT OR UPDATE ON content.article_tags FOR EACH ROW EXECUTE FUNCTION directory.fill_selected_tag_labels();
CREATE CONSTRAINT TRIGGER tags_label_ownership AFTER INSERT OR UPDATE OR DELETE ON directory.tags DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION directory.check_tag_label_ownership();
CREATE CONSTRAINT TRIGGER labels_ownership AFTER INSERT OR UPDATE OR DELETE ON directory.tag_labels DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION directory.check_tag_label_ownership();
CREATE CONSTRAINT TRIGGER sites_label_ownership AFTER INSERT OR UPDATE ON directory.sites DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION directory.check_tag_label_ownership();
CREATE CONSTRAINT TRIGGER articles_label_ownership AFTER INSERT OR UPDATE ON content.articles DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION directory.check_tag_label_ownership();
CREATE CONSTRAINT TRIGGER site_tags_label_ownership AFTER INSERT OR UPDATE ON directory.site_tags DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION directory.check_tag_label_ownership();
CREATE CONSTRAINT TRIGGER article_tags_label_ownership AFTER INSERT OR UPDATE ON content.article_tags DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION directory.check_tag_label_ownership();
CREATE CONSTRAINT TRIGGER cascades_label_ownership AFTER INSERT OR UPDATE ON directory.tag_cascades DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION directory.check_tag_label_ownership();
-- +goose StatementEnd
UPDATE directory.slug_generation_jobs SET status='cancelled',pause_code='taxonomy_labels_upgrade',resume_after=NULL,lease_token='',lease_until=NULL,revision=revision+1,updated_at=now() WHERE status NOT IN ('cancelled','completed');
GRANT SELECT,INSERT,UPDATE,DELETE ON directory.tag_labels TO api_runtime;
GRANT SELECT,INSERT,UPDATE ON directory.tag_dictionary TO api_runtime;
COMMENT ON TABLE directory.tags IS 'Semantic tag concepts; their slug is shared by all confirmed names.';
COMMENT ON COLUMN directory.tags.default_label_id IS 'Enabled default display name owned by this concept.'; -- Enabled default display name owned by this concept.
COMMENT ON TABLE directory.tag_labels IS 'Admin-confirmed names and synonyms for semantic tag concepts.';
COMMENT ON VIEW directory.tag_dictionary IS 'Default-name projection of semantic concepts for dictionary reads and atomic creation.';
COMMENT ON FUNCTION directory.write_tag_dictionary() IS 'Atomically creates concepts and names or renames their default label.';
COMMENT ON FUNCTION directory.fill_selected_tag_labels() IS 'Fills omitted selected names with the concept default.';
COMMENT ON FUNCTION directory.check_tag_label_ownership() IS 'Validates selected names and defaults against semantic concept ownership at commit.';
COMMENT ON TRIGGER tag_dictionary_write ON directory.tag_dictionary IS 'Writes authoritative concept metadata and default names together.';
COMMENT ON COLUMN directory.tag_labels.id IS 'Stable identifier of a confirmed display name.'; -- Stable identifier of a confirmed display name.
COMMENT ON COLUMN directory.tag_labels.tag_id IS 'Semantic concept owning this name.'; -- Semantic concept owning this name.
COMMENT ON COLUMN directory.tag_labels.name IS 'Chosen human-readable display name.'; -- Chosen human-readable display name.
COMMENT ON COLUMN directory.tag_labels.normalized_name IS 'Globally unique lowercase trimmed name.'; -- Globally unique lowercase trimmed name.
COMMENT ON COLUMN directory.tag_labels.is_enabled IS 'Whether this name can be selected for new assignments.'; -- Whether this name can be selected for new assignments.
COMMENT ON COLUMN directory.tag_labels.created_at IS 'Name creation time.'; -- Name creation time.
COMMENT ON COLUMN directory.tag_labels.updated_at IS 'Most recent name change time.'; -- Most recent name change time.
COMMENT ON COLUMN directory.sites.primary_label_id IS 'Chosen display name; must belong to the associated semantic concept.'; -- Chosen display name; must belong to the associated semantic concept.
COMMENT ON COLUMN directory.sites.secondary_label_id IS 'Chosen display name; must belong to the associated semantic concept.'; -- Chosen display name; must belong to the associated semantic concept.
COMMENT ON COLUMN content.articles.primary_label_id IS 'Chosen display name; must belong to the associated semantic concept.'; -- Chosen display name; must belong to the associated semantic concept.
COMMENT ON COLUMN content.articles.secondary_label_id IS 'Chosen display name; must belong to the associated semantic concept.'; -- Chosen display name; must belong to the associated semantic concept.
COMMENT ON COLUMN directory.site_tags.label_id IS 'Chosen display name; must belong to the associated semantic concept.'; -- Chosen display name; must belong to the associated semantic concept.
COMMENT ON COLUMN content.article_tags.label_id IS 'Chosen display name; must belong to the associated semantic concept.'; -- Chosen display name; must belong to the associated semantic concept.
COMMENT ON TRIGGER tags_label_ownership ON directory.tags IS 'Enforces semantic ownership of selected names.';
COMMENT ON TRIGGER labels_ownership ON directory.tag_labels IS 'Enforces semantic ownership of selected names.';
COMMENT ON TRIGGER sites_fill_labels ON directory.sites IS 'Enforces semantic ownership of selected names.';
COMMENT ON TRIGGER sites_label_ownership ON directory.sites IS 'Enforces semantic ownership of selected names.';
COMMENT ON TRIGGER articles_fill_labels ON content.articles IS 'Enforces semantic ownership of selected names.';
COMMENT ON TRIGGER articles_label_ownership ON content.articles IS 'Enforces semantic ownership of selected names.';
COMMENT ON TRIGGER site_tags_fill_labels ON directory.site_tags IS 'Enforces semantic ownership of selected names.';
COMMENT ON TRIGGER site_tags_label_ownership ON directory.site_tags IS 'Enforces semantic ownership of selected names.';
COMMENT ON TRIGGER article_tags_fill_labels ON content.article_tags IS 'Enforces semantic ownership of selected names.';
COMMENT ON TRIGGER article_tags_label_ownership ON content.article_tags IS 'Enforces semantic ownership of selected names.';
COMMENT ON TRIGGER cascades_label_ownership ON directory.tag_cascades IS 'Enforces semantic ownership of selected names.';
COMMENT ON COLUMN directory.tag_dictionary.id IS 'Default-name dictionary projection: id.'; -- Default-name dictionary projection: id.
COMMENT ON COLUMN directory.tag_dictionary.name IS 'Default-name dictionary projection: name.'; -- Default-name dictionary projection: name.
COMMENT ON COLUMN directory.tag_dictionary.normalized_name IS 'Default-name dictionary projection: normalized_name.'; -- Default-name dictionary projection: normalized_name.
COMMENT ON COLUMN directory.tag_dictionary.slug IS 'Default-name dictionary projection: slug.'; -- Default-name dictionary projection: slug.
COMMENT ON COLUMN directory.tag_dictionary.description IS 'Default-name dictionary projection: description.'; -- Default-name dictionary projection: description.
COMMENT ON COLUMN directory.tag_dictionary.is_enabled IS 'Default-name dictionary projection: is_enabled.'; -- Default-name dictionary projection: is_enabled.
COMMENT ON COLUMN directory.tag_dictionary.created_at IS 'Default-name dictionary projection: created_at.'; -- Default-name dictionary projection: created_at.
COMMENT ON COLUMN directory.tag_dictionary.updated_at IS 'Default-name dictionary projection: updated_at.'; -- Default-name dictionary projection: updated_at.
COMMENT ON COLUMN directory.tag_dictionary.default_label_id IS 'Default-name dictionary projection: default_label_id.'; -- Default-name dictionary projection: default_label_id.
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Migration 00022 is forward-only; restore the pre-upgrade database backup.'; END $$;
-- +goose StatementEnd

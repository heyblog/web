-- +goose Up

SET LOCAL check_function_bodies = false;

-- +goose StatementBegin

CREATE FUNCTION content.ensure_article_cascade_scope() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM directory.tag_cascades WHERE id=NEW.tag_cascade_id AND scope='ARTICLE'
        AND merged_into_id IS NULL AND (is_enabled OR directory.backup_restore_is_active() OR (TG_OP='UPDATE' AND OLD.tag_cascade_id=NEW.tag_cascade_id))) THEN
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

CREATE FUNCTION content.ensure_article_tag_assignment() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.role = 'TERTIARY' AND EXISTS (
        SELECT 1
          FROM content.articles AS article
          JOIN directory.tag_cascades AS cascade ON cascade.id = article.tag_cascade_id
         WHERE article.id = NEW.article_id
           AND NEW.tag_id IN (cascade.level1_tag_id, cascade.level2_tag_id)
    ) THEN
        RAISE EXCEPTION 'article tertiary tags must not repeat its selected classification path';
    END IF;
    RETURN NEW;
END;
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.ensure_site_cascade_scope() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.tag_cascade_id IS NULL THEN
        SELECT id INTO NEW.tag_cascade_id FROM directory.tag_cascades WHERE scope='SITE' AND taxonomy_key='other/topic-other-other';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM directory.tag_cascades WHERE id=NEW.tag_cascade_id AND scope='SITE'
        AND merged_into_id IS NULL AND (is_enabled OR directory.backup_restore_is_active() OR (TG_OP='UPDATE' AND OLD.tag_cascade_id=NEW.tag_cascade_id))) THEN
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

CREATE FUNCTION directory.ensure_site_tag_assignment() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.role = 'TERTIARY' AND EXISTS (
        SELECT 1
          FROM directory.sites AS site
          JOIN directory.tag_cascades AS cascade ON cascade.id = site.tag_cascade_id
         WHERE site.id = NEW.site_id
           AND NEW.tag_id IN (cascade.level1_tag_id, cascade.level2_tag_id)
    ) THEN
        RAISE EXCEPTION 'site tertiary tags must not repeat its selected classification path';
    END IF;
    RETURN NEW;
END;
$$;

-- +goose StatementEnd

CREATE TABLE content.article_tags (
    article_id uuid NOT NULL, -- Article receiving the shared tag assignment.
    tag_id uuid NOT NULL, -- Assigned shared tag.
    role text NOT NULL, -- Assignment role: TERTIARY or WARNING.
    assignment_source text DEFAULT 'MANUAL'::text NOT NULL, -- Evidence source: MANUAL, IMPORTED, or SYSTEM.
    "position" smallint,
    note text, -- Optional target-specific warning or assignment note.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Assignment creation time.
    CONSTRAINT article_tags_note_check CHECK (((note IS NULL) OR (btrim(note) <> ''::text))),
    CONSTRAINT article_tags_position_check CHECK ((((role = 'TERTIARY'::text) AND (("position" >= 1) AND ("position" <= 20))) OR ((role = 'WARNING'::text) AND ("position" IS NULL)))),
    CONSTRAINT article_tags_role_check CHECK ((role = ANY (ARRAY['TERTIARY'::text, 'WARNING'::text]))),
    CONSTRAINT article_tags_source_check CHECK ((assignment_source = ANY (ARRAY['MANUAL'::text, 'IMPORTED'::text, 'SYSTEM'::text])))
,
    CONSTRAINT article_tags_pkey PRIMARY KEY (article_id, tag_id)
);

CREATE TABLE content.articles (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 internal article primary key.
    site_id uuid NOT NULL, -- Blog site publishing the article.
    location_type text NOT NULL, -- Location storage mode: RELATIVE or EXTERNAL.
    url_ref text, -- Root-relative same-host article location without fragment.
    external_url text, -- Absolute HTTP or HTTPS location for an externally hosted article.
    url_key text NOT NULL, -- Normalized per-site article identity key.
    title text, -- Optional article title supplied by the source.
    published_at timestamp with time zone, -- Optional source publication time.
    tag_cascade_id uuid NOT NULL, -- Required ARTICLE taxonomy path.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Article discovery time.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last article metadata update time maintained by trigger.
    CONSTRAINT articles_location_check CHECK ((((location_type = 'RELATIVE'::text) AND (url_ref ~ '^/'::text) AND (url_ref !~ '#'::text) AND (external_url IS NULL) AND (url_key = url_ref)) OR ((location_type = 'EXTERNAL'::text) AND (url_ref IS NULL) AND (external_url ~ '^https?://'::text) AND (external_url !~ '#'::text) AND (url_key = external_url)))),
    CONSTRAINT articles_title_check CHECK (((title IS NULL) OR (btrim(title) <> ''::text)))
,
    CONSTRAINT articles_pkey PRIMARY KEY (id)
,
    CONSTRAINT articles_site_url_unique UNIQUE (site_id, url_key)
);

CREATE TABLE directory.tag_assignment_archive (
    id uuid DEFAULT uuidv7() NOT NULL, -- Unique archive event identifier.
    scope text NOT NULL, -- Assignment object family.
    object_id uuid NOT NULL, -- Historical object identifier.
    snapshot jsonb NOT NULL, -- Original complete association before rewriting.
    reason text NOT NULL, -- Operation that rewrote the association.
    archived_at timestamp with time zone DEFAULT now() NOT NULL, -- Archive event time.
    CONSTRAINT tag_assignment_archive_scope_check CHECK ((scope = ANY (ARRAY['SITE'::text, 'ARTICLE'::text])))
,
    CONSTRAINT tag_assignment_archive_pkey PRIMARY KEY (id)
);

CREATE TABLE directory.tag_cascades (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 shared taxonomy path primary key.
    scope text NOT NULL, -- Object family using this path: SITE or ARTICLE.
    taxonomy_key text NOT NULL, -- Stable source path key independent of environment UUIDs.
    level1_tag_id uuid NOT NULL, -- Tag occupying the primary classification role.
    level2_tag_id uuid NOT NULL, -- Tag occupying the secondary classification role.
    sort_order integer NOT NULL, -- Unique positive order within scope; expanded paths append after existing paths.
    is_enabled boolean DEFAULT true NOT NULL, -- Whether new assignments may select this classification path.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Taxonomy path creation time.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last taxonomy path update time maintained by trigger.
    merged_into_id uuid, -- Replacement path retained for historical audit references.
    CONSTRAINT cascades_merge_self_check CHECK (((merged_into_id IS NULL) OR (merged_into_id <> id))),
    CONSTRAINT tag_cascades_key_check CHECK ((taxonomy_key ~ '^[a-z0-9]+(?:-[a-z0-9]+)*/[a-z0-9]+(?:-[a-z0-9]+)*$'::text)),
    CONSTRAINT tag_cascades_scope_check CHECK ((scope = ANY (ARRAY['SITE'::text, 'ARTICLE'::text]))),
    CONSTRAINT tag_cascades_sort_check CHECK ((sort_order > 0))
,
    CONSTRAINT tag_cascades_pkey PRIMARY KEY (id)
,
    CONSTRAINT tag_cascades_scope_key_unique UNIQUE (scope, taxonomy_key)
,
    CONSTRAINT tag_cascades_scope_sort_unique UNIQUE (scope, sort_order)
);

CREATE TABLE directory.tag_identity_aliases (
    alias_id uuid NOT NULL, -- Historical tag identifier, including retained seed identifiers.
    tag_id uuid NOT NULL, -- Current tag resolved by this identity.
    system_key text, -- Historical stable import identity.
    snapshot jsonb NOT NULL, -- Historical tag snapshot or minimal import identity.
    archived_at timestamp with time zone DEFAULT now() NOT NULL, -- Identity archival time.
    alias_kind text DEFAULT 'TAG'::text NOT NULL, -- Historical identity namespace: TAG or LABEL; only TAG may have a system key.
    CONSTRAINT label_alias_no_system_key CHECK (((alias_kind = 'TAG'::text) OR (system_key IS NULL))),
    CONSTRAINT tag_identity_aliases_alias_kind_check CHECK ((alias_kind = ANY (ARRAY['TAG'::text, 'LABEL'::text])))
,
    CONSTRAINT tag_identity_aliases_pkey PRIMARY KEY (alias_kind, alias_id)
,
    CONSTRAINT tag_identity_aliases_system_key_key UNIQUE (system_key)
);

CREATE INDEX article_tags_tag_idx ON content.article_tags USING btree (tag_id, article_id);

CREATE UNIQUE INDEX article_tags_tertiary_position_unique_idx ON content.article_tags USING btree (article_id, "position") WHERE (role = 'TERTIARY'::text);

CREATE INDEX articles_site_published_idx ON content.articles USING btree (site_id, published_at DESC NULLS LAST, id);

CREATE UNIQUE INDEX tag_cascades_canonical_pair_idx ON directory.tag_cascades USING btree (scope, level1_tag_id, level2_tag_id) WHERE (merged_into_id IS NULL);

CREATE INDEX tag_cascades_level1_idx ON directory.tag_cascades USING btree (scope, level1_tag_id, sort_order);

CREATE TRIGGER article_tags_ensure_assignment BEFORE INSERT OR UPDATE ON content.article_tags FOR EACH ROW EXECUTE FUNCTION content.ensure_article_tag_assignment();

CREATE TRIGGER articles_ensure_cascade_scope BEFORE INSERT OR UPDATE OF tag_cascade_id ON content.articles FOR EACH ROW EXECUTE FUNCTION content.ensure_article_cascade_scope();

CREATE TRIGGER articles_touch_updated_at BEFORE UPDATE ON content.articles FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

CREATE TRIGGER site_tags_ensure_assignment BEFORE INSERT OR UPDATE ON directory.site_tags FOR EACH ROW EXECUTE FUNCTION directory.ensure_site_tag_assignment();

CREATE TRIGGER sites_ensure_cascade_scope BEFORE INSERT OR UPDATE OF tag_cascade_id ON directory.sites FOR EACH ROW EXECUTE FUNCTION directory.ensure_site_cascade_scope();

CREATE TRIGGER tag_cascades_touch_updated_at BEFORE UPDATE ON directory.tag_cascades FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

COMMENT ON FUNCTION content.ensure_article_cascade_scope() IS 'Enforces enabled ARTICLE cascade scope and prevents tertiary classification duplication.';

COMMENT ON FUNCTION content.ensure_article_tag_assignment() IS 'Rejects article tertiary tags that duplicate the selected classification path.';

COMMENT ON FUNCTION directory.ensure_site_cascade_scope() IS 'Enforces enabled SITE cascade scope and prevents tertiary classification duplication.';

COMMENT ON FUNCTION directory.ensure_site_tag_assignment() IS 'Rejects site tertiary tags that duplicate the selected classification path.';

COMMENT ON TABLE content.article_tags IS 'Optional ordered tertiary and warning tag assignments for articles.';

COMMENT ON COLUMN content.article_tags.article_id IS 'Article receiving the shared tag assignment.';

COMMENT ON COLUMN content.article_tags.tag_id IS 'Assigned shared tag.';

COMMENT ON COLUMN content.article_tags.role IS 'Assignment role: TERTIARY or WARNING.';

COMMENT ON COLUMN content.article_tags.assignment_source IS 'Evidence source: MANUAL, IMPORTED, or SYSTEM.';

COMMENT ON COLUMN content.article_tags."position" IS 'Ordered tertiary slot from 1 through 20; warnings are unordered.';

COMMENT ON COLUMN content.article_tags.note IS 'Optional target-specific warning or assignment note.';

COMMENT ON COLUMN content.article_tags.created_at IS 'Assignment creation time.';

COMMENT ON TABLE content.articles IS 'Article identity and metadata prepared for future ingestion without storing article bodies.';

COMMENT ON COLUMN content.articles.id IS 'UUIDv7 internal article primary key.';

COMMENT ON COLUMN content.articles.site_id IS 'Blog site publishing the article.';

COMMENT ON COLUMN content.articles.location_type IS 'Location storage mode: RELATIVE or EXTERNAL.';

COMMENT ON COLUMN content.articles.url_ref IS 'Root-relative same-host article location without fragment.';

COMMENT ON COLUMN content.articles.external_url IS 'Absolute HTTP or HTTPS location for an externally hosted article.';

COMMENT ON COLUMN content.articles.url_key IS 'Normalized per-site article identity key.';

COMMENT ON COLUMN content.articles.title IS 'Optional article title supplied by the source.';

COMMENT ON COLUMN content.articles.published_at IS 'Optional source publication time.';

COMMENT ON COLUMN content.articles.tag_cascade_id IS 'Required ARTICLE taxonomy path.';

COMMENT ON COLUMN content.articles.created_at IS 'Article discovery time.';

COMMENT ON COLUMN content.articles.updated_at IS 'Last article metadata update time maintained by trigger.';

COMMENT ON TABLE directory.tag_assignment_archive IS 'Original associations retained before taxonomy changes.';

COMMENT ON COLUMN directory.tag_assignment_archive.id IS 'Unique archive event identifier.';

COMMENT ON COLUMN directory.tag_assignment_archive.scope IS 'Assignment object family.';

COMMENT ON COLUMN directory.tag_assignment_archive.object_id IS 'Historical object identifier.';

COMMENT ON COLUMN directory.tag_assignment_archive.snapshot IS 'Original complete association before rewriting.';

COMMENT ON COLUMN directory.tag_assignment_archive.reason IS 'Operation that rewrote the association.';

COMMENT ON COLUMN directory.tag_assignment_archive.archived_at IS 'Archive event time.';

COMMENT ON TABLE directory.tag_cascades IS 'Scoped classification paths; the same tag may occupy both roles.';

COMMENT ON COLUMN directory.tag_cascades.id IS 'UUIDv7 shared taxonomy path primary key.';

COMMENT ON COLUMN directory.tag_cascades.scope IS 'Object family using this path: SITE or ARTICLE.';

COMMENT ON COLUMN directory.tag_cascades.taxonomy_key IS 'Stable source path key independent of environment UUIDs.';

COMMENT ON COLUMN directory.tag_cascades.level1_tag_id IS 'Tag occupying the primary classification role.';

COMMENT ON COLUMN directory.tag_cascades.level2_tag_id IS 'Tag occupying the secondary classification role.';

COMMENT ON COLUMN directory.tag_cascades.sort_order IS 'Unique positive order within scope; expanded paths append after existing paths.';

COMMENT ON COLUMN directory.tag_cascades.is_enabled IS 'Whether new assignments may select this classification path.';

COMMENT ON COLUMN directory.tag_cascades.created_at IS 'Taxonomy path creation time.';

COMMENT ON COLUMN directory.tag_cascades.updated_at IS 'Last taxonomy path update time maintained by trigger.';

COMMENT ON COLUMN directory.tag_cascades.merged_into_id IS 'Replacement path retained for historical audit references.';

COMMENT ON TABLE directory.tag_identity_aliases IS 'Historical tag IDs and import keys resolving directly to current tags.';

COMMENT ON COLUMN directory.tag_identity_aliases.alias_id IS 'Historical tag identifier, including retained seed identifiers.';

COMMENT ON COLUMN directory.tag_identity_aliases.tag_id IS 'Current tag resolved by this identity.';

COMMENT ON COLUMN directory.tag_identity_aliases.system_key IS 'Historical stable import identity.';

COMMENT ON COLUMN directory.tag_identity_aliases.snapshot IS 'Historical tag snapshot or minimal import identity.';

COMMENT ON COLUMN directory.tag_identity_aliases.archived_at IS 'Identity archival time.';

COMMENT ON COLUMN directory.tag_identity_aliases.alias_kind IS 'Historical identity namespace: TAG or LABEL; only TAG may have a system key.';

COMMENT ON TRIGGER article_tags_ensure_assignment ON content.article_tags IS 'Enforces article tertiary classification exclusion.';

COMMENT ON TRIGGER articles_ensure_cascade_scope ON content.articles IS 'Enforces the ARTICLE cascade invariant.';

COMMENT ON TRIGGER articles_touch_updated_at ON content.articles IS 'Maintains article update timestamps.';

COMMENT ON TRIGGER site_tags_ensure_assignment ON directory.site_tags IS 'Enforces site tertiary classification exclusion.';

COMMENT ON TRIGGER sites_ensure_cascade_scope ON directory.sites IS 'Enforces the SITE cascade invariant.';

COMMENT ON TRIGGER tag_cascades_touch_updated_at ON directory.tag_cascades IS 'Maintains taxonomy path update timestamps.';

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE content.article_tags TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE content.articles TO api_runtime;

GRANT SELECT,INSERT ON TABLE directory.tag_assignment_archive TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.tag_cascades TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.tag_identity_aliases TO api_runtime;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The consolidated baseline is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd

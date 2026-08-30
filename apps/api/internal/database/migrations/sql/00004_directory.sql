-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION directory.touch_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = clock_timestamp();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.touch_site()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.revision = OLD.revision + 1;
    NEW.updated_at = clock_timestamp();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TABLE directory.sites (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 internal site primary key.
    short_id text COLLATE "C" NOT NULL, -- Case-sensitive nine-character Base62 identifier served at the site identifier route.
    custom_id text COLLATE "C", -- Optional case-sensitive custom identifier served at the custom site route.
    name text NOT NULL, -- Public site name displayed in the directory.
    scheme text NOT NULL DEFAULT 'https', -- Canonical homepage scheme restricted to HTTP or HTTPS.
    normalized_host text NOT NULL, -- Lowercase IDNA ASCII hostname without scheme, port, path, or trailing dot.
    base_path text NOT NULL DEFAULT '/', -- Root-relative installation path without query or fragment.
    summary text NOT NULL DEFAULT '', -- Short public description of the site.
    access_scope text NOT NULL DEFAULT 'ALL', -- Expected access region: CN_ONLY, GLOBAL_ONLY, or ALL.
    visibility text NOT NULL DEFAULT 'VISIBLE', -- Directory lifecycle state: VISIBLE, HIDDEN, or REMOVED.
    visibility_reason text, -- Required explanation when the site is HIDDEN or REMOVED.
    revision bigint NOT NULL DEFAULT 1, -- Optimistic concurrency revision incremented on every update.
    joined_at timestamptz NOT NULL DEFAULT now(), -- Time the site joined the directory.
    created_at timestamptz NOT NULL DEFAULT now(), -- Database record creation time.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Last site update time maintained with revision by trigger.
    CONSTRAINT sites_short_id_check CHECK (short_id ~ '^[0-9A-Za-z]{9}$'),
    CONSTRAINT sites_custom_id_check CHECK (
        custom_id IS NULL OR (
            char_length(custom_id) BETWEEN 3 AND 32
            AND custom_id ~ '^[0-9A-Za-z][0-9A-Za-z_-]*[0-9A-Za-z]$'
            AND custom_id !~ '[_-]{2}'
        )
    ),
    CONSTRAINT sites_name_check CHECK (btrim(name) <> ''),
    CONSTRAINT sites_scheme_check CHECK (scheme IN ('http', 'https')),
    CONSTRAINT sites_host_check CHECK (
        normalized_host = lower(btrim(normalized_host))
        AND normalized_host <> ''
        AND normalized_host !~ '[/:[:space:]]'
        AND normalized_host !~ '^[0-9.]+$'
        AND right(normalized_host, 1) <> '.'
    ),
    CONSTRAINT sites_base_path_check CHECK (left(base_path, 1) = '/' AND base_path !~ '[?#]' AND base_path !~ '//'),
    CONSTRAINT sites_access_scope_check CHECK (access_scope IN ('CN_ONLY', 'GLOBAL_ONLY', 'ALL')),
    CONSTRAINT sites_visibility_check CHECK (visibility IN ('VISIBLE', 'HIDDEN', 'REMOVED')),
    CONSTRAINT sites_visibility_reason_check CHECK (
        (visibility = 'VISIBLE' AND visibility_reason IS NULL)
        OR (visibility <> 'VISIBLE' AND btrim(visibility_reason) <> '')
    ),
    CONSTRAINT sites_revision_check CHECK (revision >= 1),
    CONSTRAINT sites_joined_at_check CHECK (joined_at >= created_at)
);

CREATE UNIQUE INDEX sites_short_id_unique_idx ON directory.sites (short_id);
CREATE UNIQUE INDEX sites_custom_id_unique_idx ON directory.sites (custom_id) WHERE custom_id IS NOT NULL;
CREATE UNIQUE INDEX sites_normalized_host_unique_idx ON directory.sites (normalized_host);
CREATE INDEX sites_directory_idx ON directory.sites (visibility, joined_at DESC) WHERE visibility <> 'REMOVED';
CREATE INDEX sites_access_scope_idx ON directory.sites (access_scope, visibility);
CREATE TRIGGER sites_touch_site BEFORE UPDATE ON directory.sites
FOR EACH ROW EXECUTE FUNCTION directory.touch_site();

CREATE TABLE directory.site_feeds (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 feed candidate primary key.
    site_id uuid NOT NULL REFERENCES directory.sites(id) ON DELETE CASCADE, -- Site that publishes the feed.
    name text NOT NULL, -- Administrative feed label.
    location_type text NOT NULL, -- Location storage mode: RELATIVE or EXTERNAL.
    url_ref text, -- Root-relative same-host feed reference including meaningful query parameters.
    external_url text, -- Absolute HTTP or HTTPS feed URL for a cross-host provider.
    url_key text NOT NULL, -- Normalized per-site feed location uniqueness key.
    format text NOT NULL DEFAULT 'UNKNOWN', -- Known or detected format: UNKNOWN, RSS, ATOM, or JSON.
    is_enabled boolean NOT NULL DEFAULT true, -- Whether the feed is eligible for selection and fetching.
    is_default boolean NOT NULL DEFAULT false, -- Whether this is the single default among enabled feeds.
    created_at timestamptz NOT NULL DEFAULT now(), -- Feed candidate creation time.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Last feed configuration update time maintained by trigger.
    CONSTRAINT site_feeds_name_check CHECK (btrim(name) <> ''),
    CONSTRAINT site_feeds_location_check CHECK (
        (location_type = 'RELATIVE' AND url_ref ~ '^/' AND url_ref !~ '#' AND external_url IS NULL AND url_key = url_ref)
        OR (location_type = 'EXTERNAL' AND url_ref IS NULL AND external_url ~ '^https?://' AND external_url !~ '#' AND url_key = external_url)
    ),
    CONSTRAINT site_feeds_format_check CHECK (format IN ('UNKNOWN', 'RSS', 'ATOM', 'JSON')),
    CONSTRAINT site_feeds_default_check CHECK (NOT is_default OR is_enabled),
    CONSTRAINT site_feeds_site_url_unique UNIQUE (site_id, url_key)
);

CREATE UNIQUE INDEX site_feeds_default_unique_idx ON directory.site_feeds (site_id) WHERE is_enabled AND is_default;
CREATE INDEX site_feeds_site_enabled_idx ON directory.site_feeds (site_id, is_enabled, created_at);
CREATE TRIGGER site_feeds_touch_updated_at BEFORE UPDATE ON directory.site_feeds
FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

-- +goose StatementBegin
CREATE FUNCTION directory.enforce_default_feed()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    affected_site_id uuid;
    enabled_count integer;
    default_count integer;
BEGIN
    affected_site_id = CASE WHEN TG_OP = 'DELETE' THEN OLD.site_id ELSE NEW.site_id END;
    SELECT count(*) FILTER (WHERE is_enabled), count(*) FILTER (WHERE is_enabled AND is_default)
      INTO enabled_count, default_count
      FROM directory.site_feeds
     WHERE site_id = affected_site_id;
    IF enabled_count > 0 AND default_count <> 1 THEN
        RAISE EXCEPTION 'an enabled feed set must have exactly one default feed';
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER site_feeds_enforce_default
AFTER INSERT OR UPDATE OR DELETE ON directory.site_feeds
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION directory.enforce_default_feed();

CREATE TABLE directory.site_resources (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 site resource primary key.
    site_id uuid NOT NULL REFERENCES directory.sites(id) ON DELETE CASCADE, -- Site that owns the resource location.
    kind text NOT NULL, -- Resource purpose restricted to SITEMAP or LINK_PAGE.
    location_type text NOT NULL, -- Location storage mode: RELATIVE or EXTERNAL.
    url_ref text, -- Root-relative same-host resource reference including meaningful query parameters.
    external_url text, -- Absolute HTTP or HTTPS cross-host resource URL.
    url_key text NOT NULL, -- Normalized per-site resource location uniqueness key.
    created_at timestamptz NOT NULL DEFAULT now(), -- Resource creation time.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Last resource update time maintained by trigger.
    CONSTRAINT site_resources_kind_check CHECK (kind IN ('SITEMAP', 'LINK_PAGE')),
    CONSTRAINT site_resources_location_check CHECK (
        (location_type = 'RELATIVE' AND url_ref ~ '^/' AND url_ref !~ '#' AND external_url IS NULL AND url_key = url_ref)
        OR (location_type = 'EXTERNAL' AND url_ref IS NULL AND external_url ~ '^https?://' AND external_url !~ '#' AND url_key = external_url)
    ),
    CONSTRAINT site_resources_site_kind_unique UNIQUE (site_id, kind),
    CONSTRAINT site_resources_site_url_unique UNIQUE (site_id, url_key)
);

CREATE TRIGGER site_resources_touch_updated_at BEFORE UPDATE ON directory.site_resources
FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

CREATE TABLE directory.site_icons (
    site_id uuid PRIMARY KEY REFERENCES directory.sites(id) ON DELETE CASCADE, -- Site owning this one-to-one cached icon.
    content bytea NOT NULL, -- Cached icon bytes limited to 1 MiB.
    media_type text NOT NULL, -- Validated raster or icon MIME type.
    sha256 bytea NOT NULL, -- Raw 32-byte SHA-256 content digest used for entity tags.
    source_location_type text, -- Optional source location mode: RELATIVE or EXTERNAL.
    source_url_ref text, -- Optional root-relative same-host icon source reference.
    source_external_url text, -- Optional absolute HTTP or HTTPS cross-host icon source URL.
    fetched_at timestamptz, -- Time the source icon was last fetched successfully.
    created_at timestamptz NOT NULL DEFAULT now(), -- Cached icon creation time.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Last icon content or source update time maintained by trigger.
    CONSTRAINT site_icons_content_check CHECK (octet_length(content) BETWEEN 1 AND 1048576),
    CONSTRAINT site_icons_media_type_check CHECK (media_type IN ('image/png', 'image/jpeg', 'image/webp', 'image/gif', 'image/x-icon', 'image/vnd.microsoft.icon')),
    CONSTRAINT site_icons_sha256_check CHECK (octet_length(sha256) = 32),
    CONSTRAINT site_icons_source_check CHECK (
        (source_location_type IS NULL AND source_url_ref IS NULL AND source_external_url IS NULL)
        OR (source_location_type = 'RELATIVE' AND source_url_ref ~ '^/' AND source_url_ref !~ '#' AND source_external_url IS NULL)
        OR (source_location_type = 'EXTERNAL' AND source_url_ref IS NULL AND source_external_url ~ '^https?://' AND source_external_url !~ '#')
    ),
    CONSTRAINT site_icons_fetched_at_check CHECK (fetched_at IS NULL OR fetched_at >= created_at)
);

CREATE TRIGGER site_icons_touch_updated_at BEFORE UPDATE ON directory.site_icons
FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

CREATE TABLE directory.tags (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 global tag dictionary primary key.
    name text NOT NULL, -- Human-readable tag label.
    normalized_name text NOT NULL, -- Lowercase trimmed global semantic deduplication name.
    slug text NOT NULL, -- Stable lowercase machine key used by clients and integrations.
    description text NOT NULL DEFAULT '', -- Public tag description.
    is_enabled boolean NOT NULL DEFAULT true, -- Whether the canonical tag may be assigned or displayed.
    merged_into_id uuid REFERENCES directory.tags(id) ON DELETE RESTRICT, -- Canonical tag receiving this duplicate tag.
    merged_by uuid REFERENCES identity.users(id) ON DELETE SET NULL, -- Administrator that approved the tag merge.
    merged_at timestamptz, -- Time the tag became an alias of another tag.
    created_at timestamptz NOT NULL DEFAULT now(), -- Tag creation time.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Last tag metadata or merge update time maintained by trigger.
    CONSTRAINT tags_name_check CHECK (btrim(name) <> ''),
    CONSTRAINT tags_normalized_name_check CHECK (normalized_name = lower(btrim(normalized_name)) AND normalized_name <> ''),
    CONSTRAINT tags_slug_check CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    CONSTRAINT tags_normalized_name_unique UNIQUE (normalized_name),
    CONSTRAINT tags_slug_unique UNIQUE (slug),
    CONSTRAINT tags_merge_state_check CHECK (
        (merged_into_id IS NULL AND merged_by IS NULL AND merged_at IS NULL)
        OR (merged_into_id IS NOT NULL AND merged_at IS NOT NULL AND merged_into_id <> id)
    )
);

CREATE INDEX tags_enabled_idx ON directory.tags (is_enabled, name);
CREATE INDEX tags_merge_target_idx ON directory.tags (merged_into_id) WHERE merged_into_id IS NOT NULL;
CREATE TRIGGER tags_touch_updated_at BEFORE UPDATE ON directory.tags
FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

-- +goose StatementBegin
CREATE FUNCTION directory.prevent_tag_merge_cycle()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    cycle_found boolean;
BEGIN
    IF NEW.merged_into_id IS NULL THEN
        RETURN NEW;
    END IF;
    WITH RECURSIVE ancestors(id, merged_into_id) AS (
        SELECT id, merged_into_id FROM directory.tags WHERE id = NEW.merged_into_id
        UNION ALL
        SELECT tag.id, tag.merged_into_id FROM directory.tags AS tag
        JOIN ancestors ON tag.id = ancestors.merged_into_id
    )
    SELECT EXISTS (SELECT 1 FROM ancestors WHERE id = NEW.id) INTO cycle_found;
    IF cycle_found THEN
        RAISE EXCEPTION 'tag merge would create a cycle';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER tags_prevent_merge_cycle
BEFORE INSERT OR UPDATE OF merged_into_id ON directory.tags
FOR EACH ROW EXECUTE FUNCTION directory.prevent_tag_merge_cycle();

CREATE TABLE directory.site_tags (
    site_id uuid NOT NULL REFERENCES directory.sites(id) ON DELETE CASCADE, -- Site receiving the tag assignment.
    tag_id uuid NOT NULL REFERENCES directory.tags(id) ON DELETE RESTRICT, -- Assigned global tag.
    role text NOT NULL, -- Assignment role: PRIMARY, SECONDARY, or WARNING.
    assignment_source text NOT NULL DEFAULT 'MANUAL', -- Evidence source: MANUAL, IMPORTED, or SYSTEM.
    note text, -- Optional target-specific warning or assignment note.
    created_at timestamptz NOT NULL DEFAULT now(), -- Assignment creation time.
    PRIMARY KEY (site_id, tag_id),
    CONSTRAINT site_tags_role_check CHECK (role IN ('PRIMARY', 'SECONDARY', 'WARNING')),
    CONSTRAINT site_tags_source_check CHECK (assignment_source IN ('MANUAL', 'IMPORTED', 'SYSTEM')),
    CONSTRAINT site_tags_note_check CHECK (note IS NULL OR btrim(note) <> '')
);

CREATE UNIQUE INDEX site_tags_primary_unique_idx ON directory.site_tags (site_id) WHERE role = 'PRIMARY';
CREATE INDEX site_tags_tag_idx ON directory.site_tags (tag_id, site_id);

CREATE TABLE directory.software_components (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 software catalog primary key.
    name text NOT NULL, -- Canonical display name such as Astro or WordPress.
    normalized_name text NOT NULL, -- Lowercase trimmed global deduplication name.
    description text NOT NULL DEFAULT '', -- Public component description.
    homepage_url text, -- Optional official component homepage URL.
    repository_url text, -- Optional official source repository URL.
    is_open_source boolean NOT NULL DEFAULT false, -- Whether the component source is publicly available.
    is_enabled boolean NOT NULL DEFAULT true, -- Whether new associations may use the component.
    created_at timestamptz NOT NULL DEFAULT now(), -- Component catalog creation time.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Last component metadata update time maintained by trigger.
    CONSTRAINT software_components_name_check CHECK (btrim(name) <> ''),
    CONSTRAINT software_components_normalized_name_check CHECK (normalized_name = lower(btrim(normalized_name)) AND normalized_name <> ''),
    CONSTRAINT software_components_homepage_url_check CHECK (homepage_url IS NULL OR homepage_url ~ '^https?://'),
    CONSTRAINT software_components_repository_url_check CHECK (repository_url IS NULL OR repository_url ~ '^https?://'),
    CONSTRAINT software_components_normalized_name_unique UNIQUE (normalized_name)
);

CREATE INDEX software_components_enabled_idx ON directory.software_components (is_enabled, name);
CREATE TRIGGER software_components_touch_updated_at BEFORE UPDATE ON directory.software_components
FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

CREATE TABLE directory.software_component_dependencies (
    component_id uuid NOT NULL REFERENCES directory.software_components(id) ON DELETE CASCADE, -- Software component whose implementation uses the dependency.
    dependency_component_id uuid NOT NULL REFERENCES directory.software_components(id) ON DELETE RESTRICT, -- Referenced framework, language, runtime, or other dependency.
    role text NOT NULL, -- Dependency role: FRAMEWORK, LANGUAGE, RUNTIME, or OTHER.
    created_at timestamptz NOT NULL DEFAULT now(), -- Time the dependency relation was recorded.
    PRIMARY KEY (component_id, dependency_component_id, role),
    CONSTRAINT software_component_dependencies_distinct_check CHECK (component_id <> dependency_component_id),
    CONSTRAINT software_component_dependencies_role_check CHECK (role IN ('FRAMEWORK', 'LANGUAGE', 'RUNTIME', 'OTHER'))
);

CREATE INDEX software_component_dependencies_dependency_idx
ON directory.software_component_dependencies (dependency_component_id, role, component_id);

-- +goose StatementBegin
CREATE FUNCTION directory.prevent_software_component_dependency_cycle()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    cycle_found boolean;
BEGIN
    WITH RECURSIVE dependencies(component_id) AS (
        SELECT dependency_component_id
          FROM directory.software_component_dependencies
         WHERE component_id = NEW.dependency_component_id
        UNION
        SELECT relation.dependency_component_id
          FROM directory.software_component_dependencies AS relation
          JOIN dependencies ON relation.component_id = dependencies.component_id
    )
    SELECT EXISTS (
        SELECT 1 FROM dependencies WHERE component_id = NEW.component_id
    ) INTO cycle_found;
    IF cycle_found THEN
        RAISE EXCEPTION 'software component dependency would create a cycle';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER software_component_dependencies_prevent_cycle
BEFORE INSERT OR UPDATE OF component_id, dependency_component_id
ON directory.software_component_dependencies
FOR EACH ROW EXECUTE FUNCTION directory.prevent_software_component_dependency_cycle();

CREATE TABLE directory.site_software_components (
    site_id uuid NOT NULL REFERENCES directory.sites(id) ON DELETE CASCADE, -- Site where the component was identified.
    component_id uuid NOT NULL REFERENCES directory.software_components(id) ON DELETE RESTRICT, -- Referenced canonical software component.
    role text NOT NULL, -- Contextual role: SITE_PROGRAM, FRAMEWORK, LANGUAGE, RUNTIME, or OTHER.
    evidence_source text NOT NULL DEFAULT 'MANUAL', -- Evidence source: MANUAL, DETECTED, or IMPORTED.
    confidence numeric(4, 3), -- Optional zero-to-one confidence for detected evidence.
    identified_by uuid REFERENCES identity.users(id) ON DELETE SET NULL, -- User that manually identified or confirmed the component.
    first_identified_at timestamptz NOT NULL DEFAULT now(), -- First time this role association was identified.
    last_confirmed_at timestamptz NOT NULL DEFAULT now(), -- Most recent confirmation time for this evidence.
    PRIMARY KEY (site_id, component_id, role),
    CONSTRAINT site_software_components_role_check CHECK (role IN ('SITE_PROGRAM', 'FRAMEWORK', 'LANGUAGE', 'RUNTIME', 'OTHER')),
    CONSTRAINT site_software_components_source_check CHECK (evidence_source IN ('MANUAL', 'DETECTED', 'IMPORTED')),
    CONSTRAINT site_software_components_confidence_check CHECK (confidence IS NULL OR (evidence_source = 'DETECTED' AND confidence BETWEEN 0 AND 1)),
    CONSTRAINT site_software_components_confirmed_at_check CHECK (last_confirmed_at >= first_identified_at)
);

CREATE UNIQUE INDEX site_software_components_program_unique_idx ON directory.site_software_components (site_id) WHERE role = 'SITE_PROGRAM';
CREATE INDEX site_software_components_component_idx ON directory.site_software_components (component_id, role, site_id);

CREATE TABLE directory.site_sources (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 discovery source primary key.
    source_key text NOT NULL, -- Stable uppercase source identifier used by integrations.
    name text NOT NULL, -- Human-readable discovery source name.
    base_url text, -- Optional homepage URL of the discovery source.
    is_enabled boolean NOT NULL DEFAULT true, -- Whether new provenance records may use the source.
    created_at timestamptz NOT NULL DEFAULT now(), -- Discovery source creation time.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Last discovery source update time maintained by trigger.
    CONSTRAINT site_sources_key_check CHECK (source_key ~ '^[A-Z][A-Z0-9_]{1,63}$'),
    CONSTRAINT site_sources_name_check CHECK (btrim(name) <> ''),
    CONSTRAINT site_sources_base_url_check CHECK (base_url IS NULL OR base_url ~ '^https?://'),
    CONSTRAINT site_sources_key_unique UNIQUE (source_key)
);

CREATE TRIGGER site_sources_touch_updated_at BEFORE UPDATE ON directory.site_sources
FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

CREATE TABLE directory.site_origins (
    site_id uuid NOT NULL REFERENCES directory.sites(id) ON DELETE CASCADE, -- Site discovered or submitted through the source.
    source_id uuid NOT NULL REFERENCES directory.site_sources(id) ON DELETE RESTRICT, -- Catalog source that discovered or submitted the site.
    external_reference text, -- Optional source-side identifier or record key.
    first_discovered_at timestamptz NOT NULL DEFAULT now(), -- Earliest known discovery time from this source.
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb, -- Source-specific provenance metadata validated by the API.
    PRIMARY KEY (site_id, source_id),
    CONSTRAINT site_origins_external_reference_check CHECK (external_reference IS NULL OR btrim(external_reference) <> ''),
    CONSTRAINT site_origins_metadata_object_check CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX site_origins_source_idx ON directory.site_origins (source_id, first_discovered_at DESC);

COMMENT ON FUNCTION directory.touch_updated_at() IS 'Maintains updated_at on directory rows before update.';
COMMENT ON FUNCTION directory.touch_site() IS 'Maintains the site revision and updated_at values before update.';
COMMENT ON FUNCTION directory.enforce_default_feed() IS 'Requires exactly one default feed whenever a site has enabled feeds.';
COMMENT ON FUNCTION directory.prevent_tag_merge_cycle() IS 'Rejects tag merges that would introduce a merge cycle.';
COMMENT ON FUNCTION directory.prevent_software_component_dependency_cycle() IS 'Rejects direct software dependency changes that would introduce an indirect cycle.';
COMMENT ON TABLE directory.sites IS 'Canonical site identity, address, routing identifiers, and directory lifecycle.';
COMMENT ON TABLE directory.site_feeds IS 'Zero or more candidate feeds with one default whenever enabled feeds exist.';
COMMENT ON TABLE directory.site_resources IS 'Single-valued sitemap and friend-link-page resource locations.';
COMMENT ON TABLE directory.site_icons IS 'Cached one-to-one site icon bytes and source metadata.';
COMMENT ON TABLE directory.tags IS 'Global tag dictionary independent of assignment role.';
COMMENT ON TABLE directory.site_tags IS 'Contextual site tag assignments with one primary assignment per site.';
COMMENT ON TABLE directory.software_components IS 'Unified catalog for site programs and technology components.';
COMMENT ON TABLE directory.software_component_dependencies IS 'Direct implementation dependencies between reusable software components.';
COMMENT ON TABLE directory.site_software_components IS 'Multi-role software evidence with one site program per site.';
COMMENT ON TABLE directory.site_sources IS 'Managed catalog of sources that discover or submit sites.';
COMMENT ON TABLE directory.site_origins IS 'Many-to-many provenance between sites and discovery sources.';
COMMENT ON TRIGGER sites_touch_site ON directory.sites IS 'Maintains site revision and update time.';
COMMENT ON TRIGGER site_feeds_touch_updated_at ON directory.site_feeds IS 'Maintains feed update time.';
COMMENT ON TRIGGER site_feeds_enforce_default ON directory.site_feeds IS 'Defers enabled-feed default validation until transaction completion.';
COMMENT ON TRIGGER site_resources_touch_updated_at ON directory.site_resources IS 'Maintains resource update time.';
COMMENT ON TRIGGER site_icons_touch_updated_at ON directory.site_icons IS 'Maintains icon update time.';
COMMENT ON TRIGGER tags_touch_updated_at ON directory.tags IS 'Maintains tag update time.';
COMMENT ON TRIGGER tags_prevent_merge_cycle ON directory.tags IS 'Prevents recursive tag merge aliases.';
COMMENT ON TRIGGER software_components_touch_updated_at ON directory.software_components IS 'Maintains software component update time.';
COMMENT ON TRIGGER software_component_dependencies_prevent_cycle ON directory.software_component_dependencies IS 'Prevents recursive software component dependencies.';
COMMENT ON TRIGGER site_sources_touch_updated_at ON directory.site_sources IS 'Maintains discovery source update time.';

COMMENT ON COLUMN directory.sites.id IS 'UUIDv7 internal site primary key.';
COMMENT ON COLUMN directory.sites.short_id IS 'Case-sensitive nine-character Base62 identifier served at the site identifier route.';
COMMENT ON COLUMN directory.sites.custom_id IS 'Optional case-sensitive custom identifier served at the custom site route.';
COMMENT ON COLUMN directory.sites.name IS 'Public site name displayed in the directory.';
COMMENT ON COLUMN directory.sites.scheme IS 'Canonical homepage scheme restricted to HTTP or HTTPS.';
COMMENT ON COLUMN directory.sites.normalized_host IS 'Lowercase IDNA ASCII hostname without scheme, port, path, or trailing dot.';
COMMENT ON COLUMN directory.sites.base_path IS 'Root-relative installation path without query or fragment.';
COMMENT ON COLUMN directory.sites.summary IS 'Short public description of the site.';
COMMENT ON COLUMN directory.sites.access_scope IS 'Expected access region: CN_ONLY, GLOBAL_ONLY, or ALL.';
COMMENT ON COLUMN directory.sites.visibility IS 'Directory lifecycle state: VISIBLE, HIDDEN, or REMOVED.';
COMMENT ON COLUMN directory.sites.visibility_reason IS 'Required explanation when the site is HIDDEN or REMOVED.';
COMMENT ON COLUMN directory.sites.revision IS 'Optimistic concurrency revision incremented on every update.';
COMMENT ON COLUMN directory.sites.joined_at IS 'Time the site joined the directory.';
COMMENT ON COLUMN directory.sites.created_at IS 'Database record creation time.';
COMMENT ON COLUMN directory.sites.updated_at IS 'Last site update time maintained with revision by trigger.';
COMMENT ON COLUMN directory.site_feeds.id IS 'UUIDv7 feed candidate primary key.';
COMMENT ON COLUMN directory.site_feeds.site_id IS 'Site that publishes the feed.';
COMMENT ON COLUMN directory.site_feeds.name IS 'Administrative feed label.';
COMMENT ON COLUMN directory.site_feeds.location_type IS 'Location storage mode: RELATIVE or EXTERNAL.';
COMMENT ON COLUMN directory.site_feeds.url_ref IS 'Root-relative same-host feed reference including meaningful query parameters.';
COMMENT ON COLUMN directory.site_feeds.external_url IS 'Absolute HTTP or HTTPS feed URL for a cross-host provider.';
COMMENT ON COLUMN directory.site_feeds.url_key IS 'Normalized per-site feed location uniqueness key.';
COMMENT ON COLUMN directory.site_feeds.format IS 'Known or detected format: UNKNOWN, RSS, ATOM, or JSON.';
COMMENT ON COLUMN directory.site_feeds.is_enabled IS 'Whether the feed is eligible for selection and fetching.';
COMMENT ON COLUMN directory.site_feeds.is_default IS 'Whether this is the single default among enabled feeds.';
COMMENT ON COLUMN directory.site_feeds.created_at IS 'Feed candidate creation time.';
COMMENT ON COLUMN directory.site_feeds.updated_at IS 'Last feed configuration update time maintained by trigger.';
COMMENT ON COLUMN directory.site_resources.id IS 'UUIDv7 site resource primary key.';
COMMENT ON COLUMN directory.site_resources.site_id IS 'Site that owns the resource location.';
COMMENT ON COLUMN directory.site_resources.kind IS 'Resource purpose restricted to SITEMAP or LINK_PAGE.';
COMMENT ON COLUMN directory.site_resources.location_type IS 'Location storage mode: RELATIVE or EXTERNAL.';
COMMENT ON COLUMN directory.site_resources.url_ref IS 'Root-relative same-host resource reference including meaningful query parameters.';
COMMENT ON COLUMN directory.site_resources.external_url IS 'Absolute HTTP or HTTPS cross-host resource URL.';
COMMENT ON COLUMN directory.site_resources.url_key IS 'Normalized per-site resource location uniqueness key.';
COMMENT ON COLUMN directory.site_resources.created_at IS 'Resource creation time.';
COMMENT ON COLUMN directory.site_resources.updated_at IS 'Last resource update time maintained by trigger.';
COMMENT ON COLUMN directory.site_icons.site_id IS 'Site owning this one-to-one cached icon.';
COMMENT ON COLUMN directory.site_icons.content IS 'Cached icon bytes limited to 1 MiB.';
COMMENT ON COLUMN directory.site_icons.media_type IS 'Validated raster or icon MIME type.';
COMMENT ON COLUMN directory.site_icons.sha256 IS 'Raw 32-byte SHA-256 content digest used for entity tags.';
COMMENT ON COLUMN directory.site_icons.source_location_type IS 'Optional source location mode: RELATIVE or EXTERNAL.';
COMMENT ON COLUMN directory.site_icons.source_url_ref IS 'Optional root-relative same-host icon source reference.';
COMMENT ON COLUMN directory.site_icons.source_external_url IS 'Optional absolute HTTP or HTTPS cross-host icon source URL.';
COMMENT ON COLUMN directory.site_icons.fetched_at IS 'Time the source icon was last fetched successfully.';
COMMENT ON COLUMN directory.site_icons.created_at IS 'Cached icon creation time.';
COMMENT ON COLUMN directory.site_icons.updated_at IS 'Last icon content or source update time maintained by trigger.';
COMMENT ON COLUMN directory.tags.id IS 'UUIDv7 global tag dictionary primary key.';
COMMENT ON COLUMN directory.tags.name IS 'Human-readable tag label.';
COMMENT ON COLUMN directory.tags.normalized_name IS 'Lowercase trimmed global semantic deduplication name.';
COMMENT ON COLUMN directory.tags.slug IS 'Stable lowercase machine key used by clients and integrations.';
COMMENT ON COLUMN directory.tags.description IS 'Public tag description.';
COMMENT ON COLUMN directory.tags.is_enabled IS 'Whether the canonical tag may be assigned or displayed.';
COMMENT ON COLUMN directory.tags.merged_into_id IS 'Canonical tag receiving this duplicate tag.';
COMMENT ON COLUMN directory.tags.merged_by IS 'Administrator that approved the tag merge.';
COMMENT ON COLUMN directory.tags.merged_at IS 'Time the tag became an alias of another tag.';
COMMENT ON COLUMN directory.tags.created_at IS 'Tag creation time.';
COMMENT ON COLUMN directory.tags.updated_at IS 'Last tag metadata or merge update time maintained by trigger.';
COMMENT ON COLUMN directory.site_tags.site_id IS 'Site receiving the tag assignment.';
COMMENT ON COLUMN directory.site_tags.tag_id IS 'Assigned global tag.';
COMMENT ON COLUMN directory.site_tags.role IS 'Assignment role: PRIMARY, SECONDARY, or WARNING.';
COMMENT ON COLUMN directory.site_tags.assignment_source IS 'Evidence source: MANUAL, IMPORTED, or SYSTEM.';
COMMENT ON COLUMN directory.site_tags.note IS 'Optional target-specific warning or assignment note.';
COMMENT ON COLUMN directory.site_tags.created_at IS 'Assignment creation time.';
COMMENT ON COLUMN directory.software_components.id IS 'UUIDv7 software catalog primary key.';
COMMENT ON COLUMN directory.software_components.name IS 'Canonical display name such as Astro or WordPress.';
COMMENT ON COLUMN directory.software_components.normalized_name IS 'Lowercase trimmed global deduplication name.';
COMMENT ON COLUMN directory.software_components.description IS 'Public component description.';
COMMENT ON COLUMN directory.software_components.homepage_url IS 'Optional official component homepage URL.';
COMMENT ON COLUMN directory.software_components.repository_url IS 'Optional official source repository URL.';
COMMENT ON COLUMN directory.software_components.is_open_source IS 'Whether the component source is publicly available.';
COMMENT ON COLUMN directory.software_components.is_enabled IS 'Whether new associations may use the component.';
COMMENT ON COLUMN directory.software_components.created_at IS 'Component catalog creation time.';
COMMENT ON COLUMN directory.software_components.updated_at IS 'Last component metadata update time maintained by trigger.';
COMMENT ON COLUMN directory.software_component_dependencies.component_id IS 'Software component whose implementation uses the dependency.';
COMMENT ON COLUMN directory.software_component_dependencies.dependency_component_id IS 'Referenced framework, language, runtime, or other dependency.';
COMMENT ON COLUMN directory.software_component_dependencies.role IS 'Dependency role: FRAMEWORK, LANGUAGE, RUNTIME, or OTHER.';
COMMENT ON COLUMN directory.software_component_dependencies.created_at IS 'Time the dependency relation was recorded.';
COMMENT ON COLUMN directory.site_software_components.site_id IS 'Site where the component was identified.';
COMMENT ON COLUMN directory.site_software_components.component_id IS 'Referenced canonical software component.';
COMMENT ON COLUMN directory.site_software_components.role IS 'Contextual role: SITE_PROGRAM, FRAMEWORK, LANGUAGE, RUNTIME, or OTHER.';
COMMENT ON COLUMN directory.site_software_components.evidence_source IS 'Evidence source: MANUAL, DETECTED, or IMPORTED.';
COMMENT ON COLUMN directory.site_software_components.confidence IS 'Optional zero-to-one confidence for detected evidence.';
COMMENT ON COLUMN directory.site_software_components.identified_by IS 'User that manually identified or confirmed the component.';
COMMENT ON COLUMN directory.site_software_components.first_identified_at IS 'First time this role association was identified.';
COMMENT ON COLUMN directory.site_software_components.last_confirmed_at IS 'Most recent confirmation time for this evidence.';
COMMENT ON COLUMN directory.site_sources.id IS 'UUIDv7 discovery source primary key.';
COMMENT ON COLUMN directory.site_sources.source_key IS 'Stable uppercase source identifier used by integrations.';
COMMENT ON COLUMN directory.site_sources.name IS 'Human-readable discovery source name.';
COMMENT ON COLUMN directory.site_sources.base_url IS 'Optional homepage URL of the discovery source.';
COMMENT ON COLUMN directory.site_sources.is_enabled IS 'Whether new provenance records may use the source.';
COMMENT ON COLUMN directory.site_sources.created_at IS 'Discovery source creation time.';
COMMENT ON COLUMN directory.site_sources.updated_at IS 'Last discovery source update time maintained by trigger.';
COMMENT ON COLUMN directory.site_origins.site_id IS 'Site discovered or submitted through the source.';
COMMENT ON COLUMN directory.site_origins.source_id IS 'Catalog source that discovered or submitted the site.';
COMMENT ON COLUMN directory.site_origins.external_reference IS 'Optional source-side identifier or record key.';
COMMENT ON COLUMN directory.site_origins.first_discovered_at IS 'Earliest known discovery time from this source.';
COMMENT ON COLUMN directory.site_origins.metadata IS 'Source-specific provenance metadata validated by the API.';

-- +goose Down
DROP TABLE directory.site_origins;
DROP TABLE directory.site_sources;
DROP TABLE directory.site_software_components;
DROP TRIGGER software_component_dependencies_prevent_cycle ON directory.software_component_dependencies;
DROP FUNCTION directory.prevent_software_component_dependency_cycle();
DROP TABLE directory.software_component_dependencies;
DROP TABLE directory.software_components;
DROP TABLE directory.site_tags;
DROP TRIGGER tags_prevent_merge_cycle ON directory.tags;
DROP FUNCTION directory.prevent_tag_merge_cycle();
DROP TABLE directory.tags;
DROP TABLE directory.site_icons;
DROP TABLE directory.site_resources;
DROP TABLE directory.site_feeds;
DROP FUNCTION directory.enforce_default_feed();
DROP TABLE directory.sites;
DROP FUNCTION directory.touch_site();
DROP FUNCTION directory.touch_updated_at();

-- +goose Up

SET LOCAL check_function_bodies = false;

CREATE SCHEMA directory;

-- +goose StatementBegin

CREATE FUNCTION directory.enforce_default_feed() RETURNS trigger
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

-- +goose StatementBegin

CREATE FUNCTION directory.is_canonical_site_url(p_url text, p_host text) RETURNS boolean
    LANGUAGE sql IMMUTABLE STRICT
    SET search_path TO 'pg_catalog'
    AS $_$
    WITH parsed AS (
        SELECT substring(p_url FROM '^https?://([^/?#]+)') AS url_host,
               regexp_replace(p_url, '^https?://[^/]+', '') AS url_path
    )
    SELECT p_host ~ '^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$'
       AND p_host !~ '^[0-9.]+$'
       AND url_host = p_host
       AND url_path <> ''
       AND left(url_path, 1) = '/'
       AND url_path !~ '//'
       AND url_path !~ '[?#[:space:]]'
       AND (url_path = '/' OR right(url_path, 1) <> '/')
       AND url_path !~* '(^|/)(\.|%2e){1,2}(/|$)'
       AND url_path !~ '%($|[^0-9A-Fa-f]|[0-9A-Fa-f]($|[^0-9A-Fa-f]))'
      FROM parsed;
$_$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.prevent_software_component_dependency_cycle() RETURNS trigger
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

-- +goose StatementBegin

CREATE FUNCTION directory.touch_site() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.revision = OLD.revision + 1;
    NEW.updated_at = clock_timestamp();
    RETURN NEW;
END;
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.touch_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = clock_timestamp();
    RETURN NEW;
END;
$$;

-- +goose StatementEnd

CREATE TABLE directory.site_feeds (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 feed candidate primary key.
    site_id uuid NOT NULL, -- Site that publishes the feed.
    name text NOT NULL, -- Administrative feed label.
    location_type text NOT NULL, -- Location storage mode: RELATIVE or EXTERNAL.
    url_ref text, -- Root-relative same-host feed reference including meaningful query parameters.
    external_url text, -- Absolute HTTP or HTTPS feed URL for a cross-host provider.
    url_key text NOT NULL, -- Normalized per-site feed location uniqueness key.
    format text DEFAULT 'UNKNOWN'::text NOT NULL, -- Known or detected format: UNKNOWN, RSS, ATOM, or JSON.
    is_enabled boolean DEFAULT true NOT NULL, -- Whether the feed is eligible for selection and fetching.
    is_default boolean DEFAULT false NOT NULL, -- Whether this is the single default among enabled feeds.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Feed candidate creation time.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last feed configuration update time maintained by trigger.
    CONSTRAINT site_feeds_default_check CHECK (((NOT is_default) OR is_enabled)),
    CONSTRAINT site_feeds_format_check CHECK ((format = ANY (ARRAY['UNKNOWN'::text, 'RSS'::text, 'ATOM'::text, 'JSON'::text]))),
    CONSTRAINT site_feeds_location_check CHECK ((((location_type = 'RELATIVE'::text) AND (url_ref ~ '^/'::text) AND (url_ref !~ '#'::text) AND (external_url IS NULL) AND (url_key = url_ref)) OR ((location_type = 'EXTERNAL'::text) AND (url_ref IS NULL) AND (external_url ~ '^https?://'::text) AND (external_url !~ '#'::text) AND (url_key = external_url)))),
    CONSTRAINT site_feeds_name_check CHECK ((btrim(name) <> ''::text))
,
    CONSTRAINT site_feeds_pkey PRIMARY KEY (id)
,
    CONSTRAINT site_feeds_site_url_unique UNIQUE (site_id, url_key)
);

CREATE TABLE directory.site_icons (
    site_id uuid NOT NULL, -- Site owning this one-to-one cached icon.
    content bytea NOT NULL, -- Cached icon bytes limited to 1 MiB.
    media_type text NOT NULL, -- Validated raster or icon MIME type.
    sha256 bytea NOT NULL, -- Raw 32-byte SHA-256 content digest used for entity tags.
    source_location_type text, -- Optional source location mode: RELATIVE or EXTERNAL.
    source_url_ref text, -- Optional root-relative same-host icon source reference.
    source_external_url text, -- Optional absolute HTTP or HTTPS cross-host icon source URL.
    fetched_at timestamp with time zone, -- Time the source icon was last fetched successfully.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Cached icon creation time.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last icon content or source update time maintained by trigger.
    CONSTRAINT site_icons_content_check CHECK (((octet_length(content) >= 1) AND (octet_length(content) <= 1048576))),
    CONSTRAINT site_icons_fetched_at_check CHECK (((fetched_at IS NULL) OR (fetched_at >= created_at))),
    CONSTRAINT site_icons_media_type_check CHECK ((media_type = ANY (ARRAY['image/png'::text, 'image/jpeg'::text, 'image/webp'::text, 'image/gif'::text, 'image/x-icon'::text, 'image/vnd.microsoft.icon'::text]))),
    CONSTRAINT site_icons_sha256_check CHECK ((octet_length(sha256) = 32)),
    CONSTRAINT site_icons_source_check CHECK ((((source_location_type IS NULL) AND (source_url_ref IS NULL) AND (source_external_url IS NULL)) OR ((source_location_type = 'RELATIVE'::text) AND (source_url_ref ~ '^/'::text) AND (source_url_ref !~ '#'::text) AND (source_external_url IS NULL)) OR ((source_location_type = 'EXTERNAL'::text) AND (source_url_ref IS NULL) AND (source_external_url ~ '^https?://'::text) AND (source_external_url !~ '#'::text))))
,
    CONSTRAINT site_icons_pkey PRIMARY KEY (site_id)
);

CREATE TABLE directory.site_origins (
    site_id uuid NOT NULL, -- Site discovered or submitted through the source.
    source_id uuid NOT NULL, -- Catalog source that discovered or submitted the site.
    external_reference text, -- Optional source-side identifier or record key.
    first_discovered_at timestamp with time zone DEFAULT now() NOT NULL, -- Earliest known discovery time from this source.
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL, -- Source-specific provenance metadata validated by the API.
    CONSTRAINT site_origins_external_reference_check CHECK (((external_reference IS NULL) OR (btrim(external_reference) <> ''::text))),
    CONSTRAINT site_origins_metadata_object_check CHECK ((jsonb_typeof(metadata) = 'object'::text))
,
    CONSTRAINT site_origins_pkey PRIMARY KEY (site_id, source_id)
);

CREATE TABLE directory.site_resources (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 site resource primary key.
    site_id uuid NOT NULL, -- Site that owns the resource location.
    kind text NOT NULL, -- Resource purpose restricted to SITEMAP or LINK_PAGE.
    location_type text NOT NULL, -- Location storage mode: RELATIVE or EXTERNAL.
    url_ref text, -- Root-relative same-host resource reference including meaningful query parameters.
    external_url text, -- Absolute HTTP or HTTPS cross-host resource URL.
    url_key text NOT NULL, -- Normalized per-site resource location uniqueness key.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Resource creation time.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last resource update time maintained by trigger.
    CONSTRAINT site_resources_kind_check CHECK ((kind = ANY (ARRAY['SITEMAP'::text, 'LINK_PAGE'::text]))),
    CONSTRAINT site_resources_location_check CHECK ((((location_type = 'RELATIVE'::text) AND (url_ref ~ '^/'::text) AND (url_ref !~ '#'::text) AND (external_url IS NULL) AND (url_key = url_ref)) OR ((location_type = 'EXTERNAL'::text) AND (url_ref IS NULL) AND (external_url ~ '^https?://'::text) AND (external_url !~ '#'::text) AND (url_key = external_url))))
,
    CONSTRAINT site_resources_pkey PRIMARY KEY (id)
,
    CONSTRAINT site_resources_site_kind_unique UNIQUE (site_id, kind)
,
    CONSTRAINT site_resources_site_url_unique UNIQUE (site_id, url_key)
);

CREATE TABLE directory.site_software_components (
    site_id uuid NOT NULL, -- Site where the component was identified.
    component_id uuid NOT NULL, -- Referenced canonical software component.
    role text NOT NULL, -- Contextual role: SITE_PROGRAM, FRAMEWORK, LANGUAGE, RUNTIME, or OTHER.
    evidence_source text DEFAULT 'MANUAL'::text NOT NULL, -- Evidence source: MANUAL, DETECTED, or IMPORTED.
    confidence numeric(4,3), -- Optional zero-to-one confidence for detected evidence.
    identified_by uuid, -- User that manually identified or confirmed the component.
    first_identified_at timestamp with time zone DEFAULT now() NOT NULL, -- First time this role association was identified.
    last_confirmed_at timestamp with time zone DEFAULT now() NOT NULL, -- Most recent confirmation time for this evidence.
    CONSTRAINT site_software_components_confidence_check CHECK (((confidence IS NULL) OR ((evidence_source = 'DETECTED'::text) AND ((confidence >= (0)::numeric) AND (confidence <= (1)::numeric))))),
    CONSTRAINT site_software_components_confirmed_at_check CHECK ((last_confirmed_at >= first_identified_at)),
    CONSTRAINT site_software_components_role_check CHECK ((role = ANY (ARRAY['SITE_PROGRAM'::text, 'FRAMEWORK'::text, 'LANGUAGE'::text, 'RUNTIME'::text, 'OTHER'::text]))),
    CONSTRAINT site_software_components_source_check CHECK ((evidence_source = ANY (ARRAY['MANUAL'::text, 'DETECTED'::text, 'IMPORTED'::text])))
,
    CONSTRAINT site_software_components_pkey PRIMARY KEY (site_id, component_id, role)
);

CREATE TABLE directory.site_sources (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 discovery source primary key.
    source_key text NOT NULL, -- Stable uppercase source identifier used by integrations.
    name text NOT NULL, -- Human-readable discovery source name.
    base_url text, -- Optional homepage URL of the discovery source.
    is_enabled boolean DEFAULT true NOT NULL, -- Whether new provenance records may use the source.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Discovery source creation time.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last discovery source update time maintained by trigger.
    CONSTRAINT site_sources_base_url_check CHECK (((base_url IS NULL) OR (base_url ~ '^https?://'::text))),
    CONSTRAINT site_sources_key_check CHECK ((source_key ~ '^[A-Z][A-Z0-9_]{1,63}$'::text)),
    CONSTRAINT site_sources_name_check CHECK ((btrim(name) <> ''::text))
,
    CONSTRAINT site_sources_key_unique UNIQUE (source_key)
,
    CONSTRAINT site_sources_pkey PRIMARY KEY (id)
);

CREATE TABLE directory.site_tags (
    site_id uuid NOT NULL, -- Site receiving the tag assignment.
    tag_id uuid NOT NULL, -- Assigned global tag.
    role text NOT NULL, -- Assignment role: TERTIARY or WARNING.
    assignment_source text DEFAULT 'MANUAL'::text NOT NULL, -- Evidence source: MANUAL, IMPORTED, or SYSTEM.
    note text, -- Optional target-specific warning or assignment note.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Assignment creation time.
    "position" smallint,
    CONSTRAINT site_tags_note_check CHECK (((note IS NULL) OR (btrim(note) <> ''::text))),
    CONSTRAINT site_tags_position_check CHECK ((((role = 'TERTIARY'::text) AND (("position" >= 1) AND ("position" <= 20))) OR ((role = 'WARNING'::text) AND ("position" IS NULL)))),
    CONSTRAINT site_tags_role_check CHECK ((role = ANY (ARRAY['TERTIARY'::text, 'WARNING'::text]))),
    CONSTRAINT site_tags_source_check CHECK ((assignment_source = ANY (ARRAY['MANUAL'::text, 'IMPORTED'::text, 'SYSTEM'::text])))
,
    CONSTRAINT site_tags_pkey PRIMARY KEY (site_id, tag_id)
);

CREATE TABLE directory.sites (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 internal site primary key.
    short_id text NOT NULL COLLATE pg_catalog."C", -- Case-sensitive nine-character Base62 identifier served at the site identifier route.
    custom_id text COLLATE pg_catalog."C", -- Optional case-sensitive custom identifier served at the custom site route.
    name text NOT NULL, -- Public site name displayed in the directory.
    scheme text DEFAULT 'https'::text NOT NULL, -- Canonical homepage scheme restricted to HTTP or HTTPS.
    normalized_host text NOT NULL, -- Lowercase IDNA ASCII hostname without scheme, port, path, or trailing dot.
    base_path text DEFAULT '/'::text NOT NULL, -- Root-relative installation path without query or fragment.
    summary text DEFAULT ''::text NOT NULL, -- Short public description of the site.
    access_scope text DEFAULT 'ALL'::text NOT NULL, -- Expected access region: CN_ONLY, GLOBAL_ONLY, or ALL.
    visibility text DEFAULT 'VISIBLE'::text NOT NULL, -- Directory lifecycle state: VISIBLE, HIDDEN, or REMOVED.
    visibility_reason text, -- Required explanation when the site is HIDDEN or REMOVED.
    revision bigint DEFAULT 1 NOT NULL, -- Optimistic concurrency revision incremented on every update.
    joined_at timestamp with time zone DEFAULT now() NOT NULL, -- Time the site joined the directory.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last site update time maintained with revision by trigger.
    tag_cascade_id uuid NOT NULL, -- Required SITE taxonomy path.
    CONSTRAINT sites_access_scope_check CHECK ((access_scope = ANY (ARRAY['CN_ONLY'::text, 'GLOBAL_ONLY'::text, 'ALL'::text]))),
    CONSTRAINT sites_base_path_check CHECK ((("left"(base_path, 1) = '/'::text) AND (base_path !~ '[?#]'::text) AND (base_path !~ '//'::text))),
    CONSTRAINT sites_custom_id_check CHECK (((custom_id IS NULL) OR (((char_length(custom_id) >= 3) AND (char_length(custom_id) <= 32)) AND (custom_id ~ '^[0-9A-Za-z][0-9A-Za-z_-]*[0-9A-Za-z]$'::text) AND (custom_id !~ '[_-]{2}'::text)))),
    CONSTRAINT sites_host_check CHECK (((normalized_host = lower(btrim(normalized_host))) AND (normalized_host <> ''::text) AND (normalized_host !~ '[/:[:space:]]'::text) AND (normalized_host !~ '^[0-9.]+$'::text) AND ("right"(normalized_host, 1) <> '.'::text))),
    CONSTRAINT sites_name_check CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT sites_revision_check CHECK ((revision >= 1)),
    CONSTRAINT sites_scheme_check CHECK ((scheme = ANY (ARRAY['http'::text, 'https'::text]))),
    CONSTRAINT sites_short_id_check CHECK ((short_id ~ '^[0-9A-Za-z]{9}$'::text)),
    CONSTRAINT sites_visibility_check CHECK ((visibility = ANY (ARRAY['VISIBLE'::text, 'HIDDEN'::text, 'REMOVED'::text]))),
    CONSTRAINT sites_visibility_reason_check CHECK ((((visibility = 'VISIBLE'::text) AND (visibility_reason IS NULL)) OR ((visibility <> 'VISIBLE'::text) AND (btrim(visibility_reason) <> ''::text))))
,
    CONSTRAINT sites_pkey PRIMARY KEY (id)
);

CREATE TABLE directory.software_component_dependencies (
    component_id uuid NOT NULL, -- Software component whose implementation uses the dependency.
    dependency_component_id uuid CONSTRAINT software_component_dependencie_dependency_component_id_not_null NOT NULL, -- Referenced framework, language, runtime, or other dependency.
    role text NOT NULL, -- Dependency role: FRAMEWORK, LANGUAGE, RUNTIME, or OTHER.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Time the dependency relation was recorded.
    CONSTRAINT software_component_dependencies_distinct_check CHECK ((component_id <> dependency_component_id)),
    CONSTRAINT software_component_dependencies_role_check CHECK ((role = ANY (ARRAY['FRAMEWORK'::text, 'LANGUAGE'::text, 'RUNTIME'::text, 'OTHER'::text])))
,
    CONSTRAINT software_component_dependencies_pkey PRIMARY KEY (component_id, dependency_component_id, role)
);

CREATE TABLE directory.software_components (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 software catalog primary key.
    name text NOT NULL, -- Canonical display name such as Astro or WordPress.
    normalized_name text NOT NULL, -- Lowercase trimmed global deduplication name.
    description text DEFAULT ''::text NOT NULL, -- Public component description.
    homepage_url text, -- Optional official component homepage URL.
    repository_url text, -- Optional official source repository URL.
    is_open_source boolean DEFAULT false NOT NULL, -- Whether the component source is publicly available.
    is_enabled boolean DEFAULT true NOT NULL, -- Whether new associations may use the component.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Component catalog creation time.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last component metadata update time maintained by trigger.
    CONSTRAINT software_components_homepage_url_check CHECK (((homepage_url IS NULL) OR (homepage_url ~ '^https?://'::text))),
    CONSTRAINT software_components_name_check CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT software_components_normalized_name_check CHECK (((normalized_name = lower(btrim(normalized_name))) AND (normalized_name <> ''::text))),
    CONSTRAINT software_components_repository_url_check CHECK (((repository_url IS NULL) OR (repository_url ~ '^https?://'::text)))
,
    CONSTRAINT software_components_normalized_name_unique UNIQUE (normalized_name)
,
    CONSTRAINT software_components_pkey PRIMARY KEY (id)
);

CREATE TABLE directory.tags (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 global tag primary key.
    description text DEFAULT ''::text NOT NULL, -- Public tag description.
    is_enabled boolean DEFAULT true NOT NULL, -- Whether the canonical tag may be assigned or displayed.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Tag creation time.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last tag metadata or merge update time maintained by trigger.
    name text NOT NULL, -- Human-readable tag name.
    normalized_name text GENERATED ALWAYS AS (lower(btrim(name))) STORED NOT NULL, -- Unique lowercase trimmed tag name generated from name.
    CONSTRAINT tags_name_check CHECK (((char_length(btrim(name)) >= 1) AND (char_length(btrim(name)) <= 120)))
,
    CONSTRAINT tags_normalized_name_key UNIQUE (normalized_name)
,
    CONSTRAINT tags_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX site_feeds_default_unique_idx ON directory.site_feeds USING btree (site_id) WHERE (is_enabled AND is_default);

CREATE INDEX site_feeds_site_enabled_idx ON directory.site_feeds USING btree (site_id, is_enabled, created_at);

CREATE INDEX site_origins_source_idx ON directory.site_origins USING btree (source_id, first_discovered_at DESC);

CREATE INDEX site_software_components_component_idx ON directory.site_software_components USING btree (component_id, role, site_id);

CREATE UNIQUE INDEX site_software_components_program_unique_idx ON directory.site_software_components USING btree (site_id) WHERE (role = 'SITE_PROGRAM'::text);

CREATE INDEX site_tags_tag_idx ON directory.site_tags USING btree (tag_id, site_id);

CREATE UNIQUE INDEX site_tags_tertiary_position_unique_idx ON directory.site_tags USING btree (site_id, "position") WHERE (role = 'TERTIARY'::text);

CREATE INDEX sites_access_scope_idx ON directory.sites USING btree (access_scope, visibility);

CREATE UNIQUE INDEX sites_custom_id_unique_idx ON directory.sites USING btree (custom_id) WHERE (custom_id IS NOT NULL);

CREATE INDEX sites_directory_idx ON directory.sites USING btree (visibility, joined_at DESC) WHERE (visibility <> 'REMOVED'::text);

CREATE UNIQUE INDEX sites_normalized_host_unique_idx ON directory.sites USING btree (normalized_host);

CREATE UNIQUE INDEX sites_short_id_unique_idx ON directory.sites USING btree (short_id);

CREATE INDEX software_component_dependencies_dependency_idx ON directory.software_component_dependencies USING btree (dependency_component_id, role, component_id);

CREATE INDEX software_components_enabled_idx ON directory.software_components USING btree (is_enabled, name);

CREATE CONSTRAINT TRIGGER site_feeds_enforce_default AFTER INSERT OR DELETE OR UPDATE ON directory.site_feeds DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION directory.enforce_default_feed();

CREATE TRIGGER site_feeds_touch_updated_at BEFORE UPDATE ON directory.site_feeds FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

CREATE TRIGGER site_icons_touch_updated_at BEFORE UPDATE ON directory.site_icons FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

CREATE TRIGGER site_resources_touch_updated_at BEFORE UPDATE ON directory.site_resources FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

CREATE TRIGGER site_sources_touch_updated_at BEFORE UPDATE ON directory.site_sources FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

CREATE TRIGGER sites_touch_site BEFORE UPDATE ON directory.sites FOR EACH ROW EXECUTE FUNCTION directory.touch_site();

CREATE TRIGGER software_component_dependencies_prevent_cycle BEFORE INSERT OR UPDATE OF component_id, dependency_component_id ON directory.software_component_dependencies FOR EACH ROW EXECUTE FUNCTION directory.prevent_software_component_dependency_cycle();

CREATE TRIGGER software_components_touch_updated_at BEFORE UPDATE ON directory.software_components FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

CREATE TRIGGER tags_touch_updated_at BEFORE UPDATE ON directory.tags FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

COMMENT ON SCHEMA directory IS 'Authoritative site directory records and typed AGE graph access functions.';

COMMENT ON FUNCTION directory.enforce_default_feed() IS 'Requires exactly one default feed whenever a site has enabled feeds.';

COMMENT ON FUNCTION directory.is_canonical_site_url(p_url text, p_host text) IS 'Validates a canonical HTTP or HTTPS site registration URL for one normalized host.';

COMMENT ON FUNCTION directory.prevent_software_component_dependency_cycle() IS 'Rejects direct software dependency changes that would introduce an indirect cycle.';

COMMENT ON FUNCTION directory.touch_site() IS 'Maintains the site revision and updated_at values before update.';

COMMENT ON FUNCTION directory.touch_updated_at() IS 'Maintains updated_at on directory rows before update.';

COMMENT ON TABLE directory.site_feeds IS 'Zero or more candidate feeds with one default whenever enabled feeds exist.';

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

COMMENT ON TABLE directory.site_icons IS 'Cached one-to-one site icon bytes and source metadata.';

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

COMMENT ON TABLE directory.site_origins IS 'Many-to-many provenance between sites and discovery sources.';

COMMENT ON COLUMN directory.site_origins.site_id IS 'Site discovered or submitted through the source.';

COMMENT ON COLUMN directory.site_origins.source_id IS 'Catalog source that discovered or submitted the site.';

COMMENT ON COLUMN directory.site_origins.external_reference IS 'Optional source-side identifier or record key.';

COMMENT ON COLUMN directory.site_origins.first_discovered_at IS 'Earliest known discovery time from this source.';

COMMENT ON COLUMN directory.site_origins.metadata IS 'Source-specific provenance metadata validated by the API.';

COMMENT ON TABLE directory.site_resources IS 'Single-valued sitemap and friend-link-page resource locations.';

COMMENT ON COLUMN directory.site_resources.id IS 'UUIDv7 site resource primary key.';

COMMENT ON COLUMN directory.site_resources.site_id IS 'Site that owns the resource location.';

COMMENT ON COLUMN directory.site_resources.kind IS 'Resource purpose restricted to SITEMAP or LINK_PAGE.';

COMMENT ON COLUMN directory.site_resources.location_type IS 'Location storage mode: RELATIVE or EXTERNAL.';

COMMENT ON COLUMN directory.site_resources.url_ref IS 'Root-relative same-host resource reference including meaningful query parameters.';

COMMENT ON COLUMN directory.site_resources.external_url IS 'Absolute HTTP or HTTPS cross-host resource URL.';

COMMENT ON COLUMN directory.site_resources.url_key IS 'Normalized per-site resource location uniqueness key.';

COMMENT ON COLUMN directory.site_resources.created_at IS 'Resource creation time.';

COMMENT ON COLUMN directory.site_resources.updated_at IS 'Last resource update time maintained by trigger.';

COMMENT ON TABLE directory.site_software_components IS 'Multi-role software evidence with one site program per site.';

COMMENT ON COLUMN directory.site_software_components.site_id IS 'Site where the component was identified.';

COMMENT ON COLUMN directory.site_software_components.component_id IS 'Referenced canonical software component.';

COMMENT ON COLUMN directory.site_software_components.role IS 'Contextual role: SITE_PROGRAM, FRAMEWORK, LANGUAGE, RUNTIME, or OTHER.';

COMMENT ON COLUMN directory.site_software_components.evidence_source IS 'Evidence source: MANUAL, DETECTED, or IMPORTED.';

COMMENT ON COLUMN directory.site_software_components.confidence IS 'Optional zero-to-one confidence for detected evidence.';

COMMENT ON COLUMN directory.site_software_components.identified_by IS 'User that manually identified or confirmed the component.';

COMMENT ON COLUMN directory.site_software_components.first_identified_at IS 'First time this role association was identified.';

COMMENT ON COLUMN directory.site_software_components.last_confirmed_at IS 'Most recent confirmation time for this evidence.';

COMMENT ON TABLE directory.site_sources IS 'Managed catalog of sources that discover or submit sites.';

COMMENT ON COLUMN directory.site_sources.id IS 'UUIDv7 discovery source primary key.';

COMMENT ON COLUMN directory.site_sources.source_key IS 'Stable uppercase source identifier used by integrations.';

COMMENT ON COLUMN directory.site_sources.name IS 'Human-readable discovery source name.';

COMMENT ON COLUMN directory.site_sources.base_url IS 'Optional homepage URL of the discovery source.';

COMMENT ON COLUMN directory.site_sources.is_enabled IS 'Whether new provenance records may use the source.';

COMMENT ON COLUMN directory.site_sources.created_at IS 'Discovery source creation time.';

COMMENT ON COLUMN directory.site_sources.updated_at IS 'Last discovery source update time maintained by trigger.';

COMMENT ON TABLE directory.site_tags IS 'Contextual site tag assignments with one primary assignment per site.';

COMMENT ON COLUMN directory.site_tags.site_id IS 'Site receiving the tag assignment.';

COMMENT ON COLUMN directory.site_tags.tag_id IS 'Assigned global tag.';

COMMENT ON COLUMN directory.site_tags.role IS 'Assignment role: TERTIARY or WARNING.';

COMMENT ON COLUMN directory.site_tags.assignment_source IS 'Evidence source: MANUAL, IMPORTED, or SYSTEM.';

COMMENT ON COLUMN directory.site_tags.note IS 'Optional target-specific warning or assignment note.';

COMMENT ON COLUMN directory.site_tags.created_at IS 'Assignment creation time.';

COMMENT ON COLUMN directory.site_tags."position" IS 'Ordered tertiary slot from 1 through 20; warnings are unordered.';

COMMENT ON TABLE directory.sites IS 'Canonical site identity, address, routing identifiers, and directory lifecycle.';

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

COMMENT ON COLUMN directory.sites.updated_at IS 'Last site update time maintained with revision by trigger.';

COMMENT ON COLUMN directory.sites.tag_cascade_id IS 'Required SITE taxonomy path.';

COMMENT ON TABLE directory.software_component_dependencies IS 'Direct implementation dependencies between reusable software components.';

COMMENT ON COLUMN directory.software_component_dependencies.component_id IS 'Software component whose implementation uses the dependency.';

COMMENT ON COLUMN directory.software_component_dependencies.dependency_component_id IS 'Referenced framework, language, runtime, or other dependency.';

COMMENT ON COLUMN directory.software_component_dependencies.role IS 'Dependency role: FRAMEWORK, LANGUAGE, RUNTIME, or OTHER.';

COMMENT ON COLUMN directory.software_component_dependencies.created_at IS 'Time the dependency relation was recorded.';

COMMENT ON TABLE directory.software_components IS 'Unified catalog for site programs and technology components.';

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

COMMENT ON TABLE directory.tags IS 'Unique names; classification roles belong to paths and assignments.';

COMMENT ON COLUMN directory.tags.id IS 'UUIDv7 global tag primary key.';

COMMENT ON COLUMN directory.tags.description IS 'Public tag description.';

COMMENT ON COLUMN directory.tags.is_enabled IS 'Whether the canonical tag may be assigned or displayed.';

COMMENT ON COLUMN directory.tags.created_at IS 'Tag creation time.';

COMMENT ON COLUMN directory.tags.updated_at IS 'Last tag metadata or merge update time maintained by trigger.';

COMMENT ON COLUMN directory.tags.name IS 'Human-readable tag name.';

COMMENT ON COLUMN directory.tags.normalized_name IS 'Unique lowercase trimmed tag name generated from name.';

COMMENT ON TRIGGER site_feeds_enforce_default ON directory.site_feeds IS 'Defers enabled-feed default validation until transaction completion.';

COMMENT ON TRIGGER site_feeds_touch_updated_at ON directory.site_feeds IS 'Maintains feed update time.';

COMMENT ON TRIGGER site_icons_touch_updated_at ON directory.site_icons IS 'Maintains icon update time.';

COMMENT ON TRIGGER site_resources_touch_updated_at ON directory.site_resources IS 'Maintains resource update time.';

COMMENT ON TRIGGER site_sources_touch_updated_at ON directory.site_sources IS 'Maintains discovery source update time.';

COMMENT ON TRIGGER sites_touch_site ON directory.sites IS 'Maintains site revision and update time.';

COMMENT ON TRIGGER software_component_dependencies_prevent_cycle ON directory.software_component_dependencies IS 'Prevents recursive software component dependencies.';

COMMENT ON TRIGGER software_components_touch_updated_at ON directory.software_components IS 'Maintains software component update time.';

COMMENT ON TRIGGER tags_touch_updated_at ON directory.tags IS 'Maintains tag update time.';

GRANT USAGE ON SCHEMA directory TO api_runtime;

REVOKE ALL ON FUNCTION directory.enforce_default_feed() FROM PUBLIC;

REVOKE ALL ON FUNCTION directory.is_canonical_site_url(p_url text, p_host text) FROM PUBLIC;

REVOKE ALL ON FUNCTION directory.prevent_software_component_dependency_cycle() FROM PUBLIC;

REVOKE ALL ON FUNCTION directory.touch_site() FROM PUBLIC;

REVOKE ALL ON FUNCTION directory.touch_updated_at() FROM PUBLIC;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.site_feeds TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.site_icons TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.site_origins TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.site_resources TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.site_software_components TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.site_sources TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.site_tags TO api_runtime;

GRANT SELECT,INSERT,UPDATE ON TABLE directory.sites TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.software_component_dependencies TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.software_components TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE directory.tags TO api_runtime;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The consolidated baseline is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd

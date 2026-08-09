-- +goose Up
SET search_path = ag_catalog, "$user", public;
SELECT ag_catalog.create_graph('heyblog_directory');
SELECT ag_catalog.create_vlabel('heyblog_directory', 'SiteRef');
SELECT ag_catalog.create_elabel('heyblog_directory', 'FRIEND_LINK');

-- +goose StatementBegin
CREATE FUNCTION directory.graph_incoming_links(p_target_host text)
RETURNS TABLE (
    source_site_id uuid,
    target_url text,
    link_status text,
    created_at_ms bigint,
    updated_at_ms bigint
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
BEGIN
    RETURN QUERY EXECUTE format(
        $query$
        SELECT trim(both '"' FROM source_id::text)::uuid,
               trim(both '"' FROM edge_url::text),
               trim(both '"' FROM edge_status::text),
               edge_created::text::bigint,
               edge_updated::text::bigint
          FROM ag_catalog.cypher('heyblog_directory', $cypher$
              MATCH (source:SiteRef)-[edge:FRIEND_LINK]->(target:SiteRef {normalized_host: %s})
              RETURN source.site_id, edge.target_url, edge.status,
                     edge.created_at_ms, edge.updated_at_ms
          $cypher$) AS (
              source_id ag_catalog.agtype,
              edge_url ag_catalog.agtype,
              edge_status ag_catalog.agtype,
              edge_created ag_catalog.agtype,
              edge_updated ag_catalog.agtype
          )
        $query$,
        to_json(p_target_host)::text
    );
END;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.merge_friend_link_graph(
    p_source_site_id uuid,
    p_target_host text,
    p_target_url text,
    p_status text,
    p_created_at_ms bigint,
    p_updated_at_ms bigint
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
BEGIN
    EXECUTE format(
        $query$
        SELECT * FROM ag_catalog.cypher('heyblog_directory', $cypher$
            MATCH (source:SiteRef {site_id: %s})
            MATCH (target:SiteRef {normalized_host: %s})
            MERGE (source)-[edge:FRIEND_LINK]->(target)
            SET edge.target_url = %s,
                edge.status = CASE
                    WHEN edge.status = "ACTIVE" OR %s = "ACTIVE" THEN "ACTIVE"
                    ELSE "INACTIVE"
                END,
                edge.created_at_ms = CASE
                    WHEN edge.created_at_ms IS NULL OR edge.created_at_ms > %s THEN %s
                    ELSE edge.created_at_ms
                END,
                edge.updated_at_ms = CASE
                    WHEN edge.updated_at_ms IS NULL OR edge.updated_at_ms < %s THEN %s
                    ELSE edge.updated_at_ms
                END
            RETURN edge
        $cypher$) AS (edge ag_catalog.agtype)
        $query$,
        to_json(p_source_site_id::text)::text,
        to_json(p_target_host)::text,
        to_json(p_target_url)::text,
        to_json(p_status)::text,
        p_created_at_ms,
        p_created_at_ms,
        p_updated_at_ms,
        p_updated_at_ms
    );
END;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.upsert_site_ref(p_site_id uuid, p_normalized_host text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM directory.sites
         WHERE id = p_site_id AND normalized_host = p_normalized_host
    ) THEN
        RAISE EXCEPTION 'site reference does not match a directory site';
    END IF;

    PERFORM pg_advisory_xact_lock(hashtextextended('site-ref:' || p_normalized_host, 0));
    EXECUTE format(
        $query$
        SELECT * FROM ag_catalog.cypher('heyblog_directory', $cypher$
            MERGE (site:SiteRef {normalized_host: %s})
            SET site.site_id = %s
            RETURN site
        $cypher$) AS (site ag_catalog.agtype)
        $query$,
        to_json(p_normalized_host)::text,
        to_json(p_site_id::text)::text
    );
END;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.rewrite_friend_url(p_url text, p_scheme text, p_host text)
RETURNS text
LANGUAGE sql
IMMUTABLE
STRICT
SET search_path = pg_catalog
AS $$
    SELECT regexp_replace(p_url, '^https?://[^/?#]+', p_scheme || '://' || p_host);
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.move_site_ref(
    p_site_id uuid,
    p_old_host text,
    p_new_host text,
    p_new_scheme text
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
DECLARE
    incoming record;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM directory.sites
         WHERE id = p_site_id AND normalized_host = p_new_host AND scheme = p_new_scheme
    ) THEN
        RAISE EXCEPTION 'site move does not match the current directory site';
    END IF;

    PERFORM pg_advisory_xact_lock(hashtextextended('site-ref:' || LEAST(p_old_host, p_new_host), 0));
    PERFORM pg_advisory_xact_lock(hashtextextended('site-ref:' || GREATEST(p_old_host, p_new_host), 0));

    IF p_old_host <> p_new_host THEN
        FOR incoming IN SELECT * FROM directory.graph_incoming_links(p_new_host)
        LOOP
            PERFORM directory.merge_friend_link_graph(
                incoming.source_site_id,
                p_old_host,
                directory.rewrite_friend_url(incoming.target_url, p_new_scheme, p_new_host),
                incoming.link_status,
                incoming.created_at_ms,
                incoming.updated_at_ms
            );
        END LOOP;

        EXECUTE format(
            $query$
            SELECT * FROM ag_catalog.cypher('heyblog_directory', $cypher$
                MATCH (external:SiteRef {normalized_host: %s})
                WHERE external.site_id IS NULL
                DETACH DELETE external
                RETURN count(external)
            $cypher$) AS (deleted_count ag_catalog.agtype)
            $query$,
            to_json(p_new_host)::text
        );

        EXECUTE format(
            $query$
            SELECT * FROM ag_catalog.cypher('heyblog_directory', $cypher$
                MATCH (site:SiteRef {site_id: %s})
                SET site.normalized_host = %s
                RETURN site
            $cypher$) AS (site ag_catalog.agtype)
            $query$,
            to_json(p_site_id::text)::text,
            to_json(p_new_host)::text
        );
    END IF;

    FOR incoming IN SELECT * FROM directory.graph_incoming_links(p_new_host)
    LOOP
        PERFORM directory.merge_friend_link_graph(
            incoming.source_site_id,
            p_new_host,
            directory.rewrite_friend_url(incoming.target_url, p_new_scheme, p_new_host),
            incoming.link_status,
            incoming.created_at_ms,
            incoming.updated_at_ms
        );
    END LOOP;
END;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.upsert_friend_link(
    p_source_site_id uuid,
    p_target_url text,
    p_target_host text,
    p_status text DEFAULT 'ACTIVE'
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
DECLARE
    source_host text;
    event_ms bigint;
BEGIN
    IF p_status NOT IN ('ACTIVE', 'INACTIVE') THEN
        RAISE EXCEPTION 'friend-link status must be ACTIVE or INACTIVE';
    END IF;
    IF p_target_host <> lower(btrim(p_target_host)) OR p_target_host = '' THEN
        RAISE EXCEPTION 'friend-link target host must be normalized';
    END IF;
    IF substring(p_target_url FROM '^https?://([^/?#]+)') IS DISTINCT FROM p_target_host
       OR p_target_url ~ '#' THEN
        RAISE EXCEPTION 'friend-link URL must be an absolute normalized URL without a fragment';
    END IF;

    SELECT normalized_host INTO source_host FROM directory.sites WHERE id = p_source_site_id;
    IF source_host IS NULL THEN
        RAISE EXCEPTION 'friend-link source site does not exist';
    END IF;
    IF source_host = p_target_host THEN
        RAISE EXCEPTION 'friend-link self edges are not allowed';
    END IF;

    PERFORM pg_advisory_xact_lock(hashtextextended('friend-link:' || p_source_site_id::text || ':' || p_target_host, 0));
    EXECUTE format(
        $query$
        SELECT * FROM ag_catalog.cypher('heyblog_directory', $cypher$
            MERGE (target:SiteRef {normalized_host: %s})
            RETURN target
        $cypher$) AS (target ag_catalog.agtype)
        $query$,
        to_json(p_target_host)::text
    );

    event_ms = (extract(epoch FROM clock_timestamp()) * 1000)::bigint;
    PERFORM directory.merge_friend_link_graph(
        p_source_site_id,
        p_target_host,
        p_target_url,
        p_status,
        event_ms,
        event_ms
    );
END;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.list_friend_links(
    p_source_site_id uuid,
    p_include_inactive boolean DEFAULT false
)
RETURNS TABLE (
    target_site_id uuid,
    target_host text,
    target_url text,
    link_status text,
    is_reciprocal boolean,
    created_at_ms bigint,
    updated_at_ms bigint
)
LANGUAGE plpgsql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
BEGIN
    RETURN QUERY EXECUTE format(
        $query$
        WITH graph_links AS (
            SELECT NULLIF(trim(both '"' FROM graph_target_id::text), 'null')::uuid AS resolved_site_id,
                   trim(both '"' FROM graph_target_host::text) AS resolved_host,
                   trim(both '"' FROM graph_target_url::text) AS resolved_url,
                   trim(both '"' FROM graph_status::text) AS resolved_status,
                   graph_reciprocal::text::boolean AS resolved_reciprocal,
                   graph_created::text::bigint AS resolved_created,
                   graph_updated::text::bigint AS resolved_updated
              FROM ag_catalog.cypher('heyblog_directory', $cypher$
                  MATCH (source:SiteRef {site_id: %s})-[edge:FRIEND_LINK]->(target:SiteRef)
                  OPTIONAL MATCH (target)-[reverse:FRIEND_LINK]->(source)
                  WHERE reverse.status = "ACTIVE"
                  RETURN target.site_id, target.normalized_host, edge.target_url, edge.status,
                         count(reverse) > 0, edge.created_at_ms, edge.updated_at_ms
              $cypher$) AS (
                  graph_target_id ag_catalog.agtype,
                  graph_target_host ag_catalog.agtype,
                  graph_target_url ag_catalog.agtype,
                  graph_status ag_catalog.agtype,
                  graph_reciprocal ag_catalog.agtype,
                  graph_created ag_catalog.agtype,
                  graph_updated ag_catalog.agtype
              )
        )
        SELECT link.resolved_site_id, link.resolved_host, link.resolved_url,
               link.resolved_status, link.resolved_reciprocal,
               link.resolved_created, link.resolved_updated
          FROM graph_links AS link
          LEFT JOIN directory.sites AS target_site ON target_site.id = link.resolved_site_id
         WHERE (%L OR link.resolved_status = 'ACTIVE')
           AND (link.resolved_site_id IS NULL OR target_site.visibility = 'VISIBLE')
         ORDER BY link.resolved_host, link.resolved_url
        $query$,
        to_json(p_source_site_id::text)::text,
        p_include_inactive
    );
END;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.delete_site_ref(p_site_id uuid)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
BEGIN
    IF EXISTS (SELECT 1 FROM directory.sites WHERE id = p_site_id) THEN
        RAISE EXCEPTION 'cannot delete the graph reference of an existing site';
    END IF;
    EXECUTE format(
        $query$
        SELECT * FROM ag_catalog.cypher('heyblog_directory', $cypher$
            MATCH (site:SiteRef {site_id: %s})
            DETACH DELETE site
            RETURN count(site)
        $cypher$) AS (deleted_count ag_catalog.agtype)
        $query$,
        to_json(p_site_id::text)::text
    );
END;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.sync_site_ref_insert()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
BEGIN
    PERFORM directory.upsert_site_ref(NEW.id, NEW.normalized_host);
    RETURN NEW;
END;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.sync_site_ref_move()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
BEGIN
    PERFORM directory.move_site_ref(NEW.id, OLD.normalized_host, NEW.normalized_host, NEW.scheme);
    RETURN NEW;
END;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.prevent_site_delete()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
    RAISE EXCEPTION 'sites use HIDDEN or REMOVED lifecycle states and cannot be deleted';
END;
$function$;
-- +goose StatementEnd

CREATE TRIGGER sites_sync_graph_insert AFTER INSERT ON directory.sites
FOR EACH ROW EXECUTE FUNCTION directory.sync_site_ref_insert();
CREATE TRIGGER sites_sync_graph_move AFTER UPDATE OF scheme, normalized_host ON directory.sites
FOR EACH ROW
WHEN (OLD.scheme IS DISTINCT FROM NEW.scheme OR OLD.normalized_host IS DISTINCT FROM NEW.normalized_host)
EXECUTE FUNCTION directory.sync_site_ref_move();
CREATE TRIGGER sites_prevent_delete BEFORE DELETE ON directory.sites
FOR EACH ROW EXECUTE FUNCTION directory.prevent_site_delete();

COMMENT ON FUNCTION directory.graph_incoming_links(text) IS 'Internal typed reader for FRIEND_LINK edges targeting one normalized host.';
COMMENT ON FUNCTION directory.merge_friend_link_graph(uuid, text, text, text, bigint, bigint) IS 'Internal AGE edge merge preserving active status and timestamp conflict rules.';
COMMENT ON FUNCTION directory.upsert_site_ref(uuid, text) IS 'Creates or promotes one SiteRef vertex for an authoritative directory site.';
COMMENT ON FUNCTION directory.rewrite_friend_url(text, text, text) IS 'Rewrites only the scheme and host of a normalized absolute friend-link URL.';
COMMENT ON FUNCTION directory.move_site_ref(uuid, text, text, text) IS 'Moves a registered SiteRef, merges a matching external vertex, and rewrites incoming URLs.';
COMMENT ON FUNCTION directory.upsert_friend_link(uuid, text, text, text) IS 'Creates or updates one directed FRIEND_LINK edge through the typed AGE boundary.';
COMMENT ON FUNCTION directory.list_friend_links(uuid, boolean) IS 'Lists typed friend links and derives reciprocity from a reverse active edge.';
COMMENT ON FUNCTION directory.delete_site_ref(uuid) IS 'Deletes an orphan registered SiteRef during privileged maintenance only.';
COMMENT ON FUNCTION directory.sync_site_ref_insert() IS 'Synchronizes a newly inserted directory site into AGE in the same transaction.';
COMMENT ON FUNCTION directory.sync_site_ref_move() IS 'Synchronizes a canonical site address change into AGE in the same transaction.';
COMMENT ON FUNCTION directory.prevent_site_delete() IS 'Rejects hard deletion so site lifecycle and graph history remain stable.';
COMMENT ON TRIGGER sites_sync_graph_insert ON directory.sites IS 'Creates or promotes the matching SiteRef after site insertion.';
COMMENT ON TRIGGER sites_sync_graph_move ON directory.sites IS 'Moves the matching SiteRef after canonical address changes.';
COMMENT ON TRIGGER sites_prevent_delete ON directory.sites IS 'Requires HIDDEN or REMOVED instead of hard deletion.';

-- +goose Down
DROP TRIGGER sites_prevent_delete ON directory.sites;
DROP TRIGGER sites_sync_graph_move ON directory.sites;
DROP TRIGGER sites_sync_graph_insert ON directory.sites;
DROP FUNCTION directory.prevent_site_delete();
DROP FUNCTION directory.sync_site_ref_move();
DROP FUNCTION directory.sync_site_ref_insert();
DROP FUNCTION directory.delete_site_ref(uuid);
DROP FUNCTION directory.list_friend_links(uuid, boolean);
DROP FUNCTION directory.upsert_friend_link(uuid, text, text, text);
DROP FUNCTION directory.move_site_ref(uuid, text, text, text);
DROP FUNCTION directory.rewrite_friend_url(text, text, text);
DROP FUNCTION directory.upsert_site_ref(uuid, text);
DROP FUNCTION directory.merge_friend_link_graph(uuid, text, text, text, bigint, bigint);
DROP FUNCTION directory.graph_incoming_links(text);
SELECT ag_catalog.drop_graph('heyblog_directory', true);

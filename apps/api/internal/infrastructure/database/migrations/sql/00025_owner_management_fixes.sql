-- +goose Up
ALTER TABLE directory.owner_friend_link_requests
    ADD COLUMN notify_by_email boolean NOT NULL DEFAULT false; -- Whether the recommending account wants an audit decision email.
COMMENT ON COLUMN directory.owner_friend_link_requests.notify_by_email IS 'Whether the recommending account wants an audit decision email.';

-- +goose StatementBegin
CREATE FUNCTION directory.list_owner_friend_links(
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
    IF NOT EXISTS (
        SELECT 1 FROM directory.sites
         WHERE id = p_source_site_id AND visibility <> 'REMOVED'
    ) THEN
        RETURN;
    END IF;

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
              FROM ag_catalog.cypher('directory_graph', $cypher$
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
         WHERE (%L OR link.resolved_status = 'ACTIVE')
         ORDER BY link.resolved_host, link.resolved_url
        $query$,
        to_json(p_source_site_id::text)::text,
        p_include_inactive
    );
END;
$function$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION directory.list_owner_friend_links(uuid, boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION directory.list_owner_friend_links(uuid, boolean) TO api_runtime;
COMMENT ON FUNCTION directory.list_owner_friend_links(uuid, boolean) IS 'Reads all existing friend edges for authenticated owner workflows, including hidden sources and unavailable targets.';

-- +goose StatementBegin
CREATE FUNCTION directory.remove_owner_friend_link(p_source_site_id uuid, p_target_host text)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $$
DECLARE
    existing_url text;
    event_ms bigint;
BEGIN
    PERFORM 1 FROM directory.sites WHERE id = p_source_site_id FOR UPDATE;
    PERFORM pg_advisory_xact_lock(hashtextextended('site-ref:' || p_target_host, 0));
    PERFORM pg_advisory_xact_lock(hashtextextended('friend-link:' || p_source_site_id::text || ':' || p_target_host, 0));
    SELECT target_url INTO existing_url FROM directory.list_owner_friend_links(p_source_site_id, true)
      WHERE target_host = p_target_host AND link_status = 'ACTIVE';
    IF existing_url IS NULL THEN RETURN false; END IF;
    event_ms := (extract(epoch FROM clock_timestamp()) * 1000)::bigint;
    PERFORM directory.merge_friend_link_graph(p_source_site_id, p_target_host, existing_url, 'INACTIVE', event_ms, event_ms);
    RETURN true;
END;
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION directory.remove_owner_friend_link(uuid, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION directory.remove_owner_friend_link(uuid, text) TO api_runtime;
COMMENT ON FUNCTION directory.remove_owner_friend_link(uuid, text) IS 'Deactivates an existing registered or external friend edge without creating an unreviewed target.';

-- +goose Down
DROP FUNCTION directory.remove_owner_friend_link(uuid, text);
DROP FUNCTION directory.list_owner_friend_links(uuid, boolean);
ALTER TABLE directory.owner_friend_link_requests DROP COLUMN notify_by_email;

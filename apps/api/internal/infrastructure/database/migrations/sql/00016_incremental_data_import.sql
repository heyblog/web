-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION directory.import_friend_link_pairs()
RETURNS TABLE (source_site_id uuid, target_host text)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
    SELECT NULLIF(trim(both '"' FROM source_id::text), 'null')::uuid,
           trim(both '"' FROM graph_target_host::text)
      FROM ag_catalog.cypher('directory_graph', $cypher$
          MATCH (source:SiteRef)-[edge:FRIEND_LINK]->(target:SiteRef)
          RETURN source.site_id, target.normalized_host
      $cypher$) AS (source_id ag_catalog.agtype, graph_target_host ag_catalog.agtype);
$function$;
-- +goose StatementEnd

COMMENT ON FUNCTION directory.import_friend_link_pairs() IS
'Internal incremental import reader for all directed friend-link pairs, including inactive links and non-visible sites.';
REVOKE ALL ON FUNCTION directory.import_friend_link_pairs() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION directory.import_friend_link_pairs() TO api_runtime;

-- +goose Down
DROP FUNCTION directory.import_friend_link_pairs();

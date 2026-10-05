-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION directory.public_friend_graph(p_center_id uuid DEFAULT NULL)
RETURNS jsonb
LANGUAGE plpgsql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
DECLARE
    center_short_id text;
    center_visibility text;
    result jsonb;
BEGIN
    IF p_center_id IS NOT NULL THEN
        SELECT short_id, visibility INTO center_short_id, center_visibility
          FROM directory.sites WHERE id = p_center_id;
        IF center_short_id IS NULL OR center_visibility = 'REMOVED' THEN
            RETURN NULL;
        END IF;
        IF center_visibility = 'HIDDEN' THEN
            RETURN jsonb_build_object('nodes', '[]'::jsonb, 'edges', '[]'::jsonb,
                'centerId', 'site:' || center_short_id,
                'stats', jsonb_build_object('nodes', 0, 'edges', 0, 'reciprocalPairs', 0));
        END IF;
    END IF;

    EXECUTE format($query$
        WITH raw_edges AS MATERIALIZED (
            SELECT NULLIF(trim(both '"' FROM source_id::text), 'null')::uuid AS source_id,
                   NULLIF(trim(both '"' FROM target_id::text), 'null')::uuid AS target_id,
                   trim(both '"' FROM target_host::text) AS target_host,
                   trim(both '"' FROM target_url::text) AS target_url
              FROM ag_catalog.cypher('directory_graph', $cypher$
                MATCH (source:SiteRef)-[edge:FRIEND_LINK]->(target:SiteRef)
                WHERE edge.status = 'ACTIVE' %s
                RETURN source.site_id, target.site_id, target.normalized_host, edge.target_url
              $cypher$) AS (source_id ag_catalog.agtype, target_id ag_catalog.agtype,
                            target_host ag_catalog.agtype, target_url ag_catalog.agtype)
        ), visible_edges AS MATERIALIZED (
            SELECT DISTINCT 'site:' || source.short_id AS source,
                   CASE WHEN target.id IS NULL THEN 'external:' || edge.target_host
                        ELSE 'site:' || target.short_id END AS target,
                   edge.target_host, edge.target_url
              FROM raw_edges AS edge
              JOIN directory.sites AS source ON source.id = edge.source_id AND source.visibility = 'VISIBLE'
              LEFT JOIN directory.sites AS target ON target.id = edge.target_id
             WHERE edge.target_id IS NULL OR target.visibility = 'VISIBLE'
        ), nodes AS (
            SELECT 'site:' || site.short_id AS id, site.name, site.normalized_host AS host,
                   site.scheme || '://' || site.normalized_host || site.base_path AS homepage_url,
                   site.short_id, site.custom_id
              FROM directory.sites AS site
             WHERE site.visibility = 'VISIBLE' AND (%L::uuid IS NULL OR site.id = %L::uuid
                OR EXISTS (SELECT 1 FROM visible_edges AS edge
                            WHERE edge.source = 'site:' || site.short_id OR edge.target = 'site:' || site.short_id))
            UNION ALL
            SELECT edge.target, edge.target_host, edge.target_host, min(edge.target_url), NULL, NULL
              FROM visible_edges AS edge WHERE edge.target LIKE 'external:%%'
             GROUP BY edge.target, edge.target_host
        ), edges AS (
            SELECT edge.source, edge.target,
                   EXISTS (SELECT 1 FROM visible_edges AS reverse
                            WHERE reverse.source = edge.target AND reverse.target = edge.source) AS reciprocal
              FROM visible_edges AS edge
        )
        SELECT jsonb_build_object(
            'nodes', COALESCE((SELECT jsonb_agg(jsonb_build_object('id', id, 'name', name, 'host', host,
                'homepageUrl', homepage_url, 'shortId', short_id, 'customId', custom_id) ORDER BY id) FROM nodes), '[]'::jsonb),
            'edges', COALESCE((SELECT jsonb_agg(jsonb_build_object('source', source, 'target', target,
                'reciprocal', reciprocal) ORDER BY source, target) FROM edges), '[]'::jsonb),
            'centerId', %L::text,
            'stats', jsonb_build_object('nodes', (SELECT count(*) FROM nodes),
                'edges', (SELECT count(*) FROM edges),
                'reciprocalPairs', (SELECT count(*) FROM edges WHERE reciprocal AND source < target)))
    $query$,
        CASE WHEN p_center_id IS NULL THEN '' ELSE format(
            'AND (source.site_id = %s OR target.site_id = %s)',
            to_json(p_center_id::text)::text, to_json(p_center_id::text)::text) END,
        p_center_id, p_center_id,
        CASE WHEN p_center_id IS NULL THEN NULL ELSE 'site:' || center_short_id END
    ) INTO result;
    RETURN result;
END;
$function$;
-- +goose StatementEnd

COMMENT ON FUNCTION directory.public_friend_graph(uuid) IS
'Returns one consistent public graph snapshot of ACTIVE FRIEND_LINK edges and VISIBLE SiteRef sites; null center includes isolated sites and external targets, a center restricts reads to incident edges.';
REVOKE ALL ON FUNCTION directory.public_friend_graph(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION directory.public_friend_graph(uuid) TO api_runtime;

-- +goose Down
DROP FUNCTION directory.public_friend_graph(uuid);

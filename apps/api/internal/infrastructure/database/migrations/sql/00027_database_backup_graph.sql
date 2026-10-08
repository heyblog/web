-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION directory.backup_graph_rows(p_kind text)
RETURNS SETOF jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
DECLARE
    graph_row record;
    properties jsonb;
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
        WHERE n.nspname='directory_graph' AND c.relkind IN ('r','p')
          AND c.relname NOT IN ('_ag_label_vertex','_ag_label_edge','SiteRef','FRIEND_LINK')
    ) THEN
        RAISE EXCEPTION 'unsupported backup graph label' USING ERRCODE='22023';
    END IF;
    IF EXISTS (SELECT 1 FROM ONLY directory_graph._ag_label_vertex)
       OR EXISTS (SELECT 1 FROM ONLY directory_graph._ag_label_edge) THEN
        RAISE EXCEPTION 'unsupported unlabeled backup graph data' USING ERRCODE='22023';
    END IF;
    IF p_kind='vertices' THEN
        FOR graph_row IN EXECUTE $query$
            SELECT p::text::jsonb AS properties FROM ag_catalog.cypher('directory_graph', $cypher$
                MATCH (v:SiteRef) RETURN properties(v)
            $cypher$) AS (p ag_catalog.agtype)
            ORDER BY (p::text::jsonb->>'normalized_host') COLLATE "C"
        $query$ LOOP
            properties:=graph_row.properties;
            IF properties-ARRAY['normalized_host','site_id']<>'{}'::jsonb THEN
                RAISE EXCEPTION 'unsupported backup vertex property' USING ERRCODE='22023';
            END IF;
            RETURN NEXT jsonb_build_object('normalized_host',properties->'normalized_host','site_id',properties->'site_id');
        END LOOP;
    ELSIF p_kind='edges' THEN
        FOR graph_row IN EXECUTE $query$
            SELECT to_jsonb(s::text) AS source_host,to_jsonb(t::text) AS target_host,p::text::jsonb AS properties
            FROM ag_catalog.cypher('directory_graph', $cypher$
                MATCH (s:SiteRef)-[e:FRIEND_LINK]->(t:SiteRef)
                RETURN s.normalized_host,t.normalized_host,properties(e)
            $cypher$) AS (s ag_catalog.agtype,t ag_catalog.agtype,p ag_catalog.agtype)
            ORDER BY s::text COLLATE "C",t::text COLLATE "C"
        $query$ LOOP
            properties:=graph_row.properties;
            IF properties-ARRAY['target_url','status','created_at_ms','updated_at_ms']<>'{}'::jsonb THEN
                RAISE EXCEPTION 'unsupported backup edge property' USING ERRCODE='22023';
            END IF;
            RETURN NEXT jsonb_build_object('source_host',graph_row.source_host,'target_host',graph_row.target_host,
                'target_url',properties->'target_url','status',properties->'status',
                'created_at_ms',(properties->>'created_at_ms')::bigint::text,
                'updated_at_ms',(properties->>'updated_at_ms')::bigint::text);
        END LOOP;
    ELSE
        RAISE EXCEPTION 'unsupported backup graph dataset' USING ERRCODE='22023';
    END IF;
END;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.backup_restore_graph_begin()
RETURNS void
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
BEGIN
    IF NOT directory.backup_restore_is_active() THEN
        RAISE EXCEPTION 'database restore transaction required' USING ERRCODE='42501';
    END IF;
    LOCK TABLE directory_graph."_ag_label_vertex",directory_graph."_ag_label_edge",
        directory_graph."SiteRef",directory_graph."FRIEND_LINK" IN ACCESS EXCLUSIVE MODE;
    CREATE TEMP TABLE heyblog_restore_graph_vertices(host text PRIMARY KEY) ON COMMIT DROP;
    CREATE TEMP TABLE heyblog_restore_graph_edges(source_host text,target_host text,PRIMARY KEY(source_host,target_host)) ON COMMIT DROP;
    REVOKE ALL ON pg_temp.heyblog_restore_graph_vertices,pg_temp.heyblog_restore_graph_edges FROM PUBLIC,api_runtime;
    EXECUTE $query$
        SELECT * FROM ag_catalog.cypher('directory_graph', $cypher$
            MATCH (v) DETACH DELETE v RETURN count(v)
        $cypher$) AS (removed ag_catalog.agtype)
    $query$;
END;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.backup_restore_graph_row(p_kind text,p_row jsonb)
RETURNS void
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
DECLARE
    host text;
    site_id uuid;
    source_host text;
    target_host text;
    target_url text;
    canonical_url text;
    edge_status text;
    created_ms bigint;
    updated_ms bigint;
    matching_count bigint;
BEGIN
    IF NOT directory.backup_restore_is_active() THEN
        RAISE EXCEPTION 'database restore transaction required' USING ERRCODE='42501';
    END IF;
    IF jsonb_typeof(p_row) IS DISTINCT FROM 'object' THEN
        RAISE EXCEPTION 'invalid backup graph row' USING ERRCODE='22023';
    END IF;
    IF p_kind='vertices' THEN
        IF p_row-ARRAY['normalized_host','site_id']<>'{}'::jsonb OR NOT p_row ?& ARRAY['normalized_host','site_id'] THEN
            RAISE EXCEPTION 'invalid backup vertex fields' USING ERRCODE='22023';
        END IF;
        host:=p_row->>'normalized_host';
        site_id:=(p_row->>'site_id')::uuid;
        IF jsonb_typeof(p_row->'normalized_host') IS DISTINCT FROM 'string'
           OR host IS NULL OR NOT directory.is_canonical_site_url('https://'||host||'/',host)
           OR jsonb_typeof(p_row->'site_id') NOT IN ('null','string') THEN
            RAISE EXCEPTION 'invalid backup vertex identity' USING ERRCODE='22023';
        END IF;
        INSERT INTO pg_temp.heyblog_restore_graph_vertices VALUES(host);
        IF site_id IS NOT NULL THEN
            IF NOT EXISTS(SELECT 1 FROM directory.sites s WHERE s.id=site_id AND s.normalized_host=host) THEN
                RAISE EXCEPTION 'backup vertex does not match restored site' USING ERRCODE='22023';
            END IF;
            EXECUTE format($query$
                SELECT n::text::bigint FROM ag_catalog.cypher('directory_graph', $cypher$
                    MATCH (v:SiteRef {normalized_host:%s,site_id:%s}) RETURN count(v)
                $cypher$) AS (n ag_catalog.agtype)
            $query$,to_json(host)::text,to_json(site_id::text)::text) INTO matching_count;
            IF matching_count<>1 THEN
                RAISE EXCEPTION 'registered backup vertex is inconsistent' USING ERRCODE='22023';
            END IF;
        ELSE
            IF EXISTS(SELECT 1 FROM directory.sites s WHERE s.normalized_host=host) THEN
                RAISE EXCEPTION 'external backup vertex matches registered site' USING ERRCODE='22023';
            END IF;
            EXECUTE format($query$
                SELECT * FROM ag_catalog.cypher('directory_graph', $cypher$
                    CREATE (v:SiteRef {normalized_host:%s}) RETURN v
                $cypher$) AS (v ag_catalog.agtype)
            $query$,to_json(host)::text);
        END IF;
    ELSIF p_kind='edges' THEN
        IF p_row-ARRAY['source_host','target_host','target_url','status','created_at_ms','updated_at_ms']<>'{}'::jsonb
           OR NOT p_row ?& ARRAY['source_host','target_host','target_url','status','created_at_ms','updated_at_ms'] THEN
            RAISE EXCEPTION 'invalid backup edge fields' USING ERRCODE='22023';
        END IF;
        source_host:=p_row->>'source_host'; target_host:=p_row->>'target_host'; target_url:=p_row->>'target_url';
        edge_status:=p_row->>'status'; created_ms:=(p_row->>'created_at_ms')::bigint; updated_ms:=(p_row->>'updated_at_ms')::bigint;
        IF jsonb_typeof(p_row->'source_host') IS DISTINCT FROM 'string'
           OR jsonb_typeof(p_row->'target_host') IS DISTINCT FROM 'string'
           OR jsonb_typeof(p_row->'target_url') IS DISTINCT FROM 'string'
           OR jsonb_typeof(p_row->'status') IS DISTINCT FROM 'string'
           OR jsonb_typeof(p_row->'created_at_ms') IS DISTINCT FROM 'string'
           OR jsonb_typeof(p_row->'updated_at_ms') IS DISTINCT FROM 'string'
           OR source_host IS NULL OR target_host IS NULL OR target_url IS NULL OR source_host=target_host
           OR edge_status IS NULL OR edge_status NOT IN ('ACTIVE','INACTIVE')
           OR created_ms IS NULL OR updated_ms IS NULL OR created_ms>updated_ms
           OR NOT directory.is_canonical_site_url(target_url,target_host)
           OR NOT EXISTS(SELECT 1 FROM pg_temp.heyblog_restore_graph_vertices v WHERE v.host=source_host)
           OR NOT EXISTS(SELECT 1 FROM pg_temp.heyblog_restore_graph_vertices v WHERE v.host=target_host) THEN
            RAISE EXCEPTION 'invalid backup edge identity or properties' USING ERRCODE='22023';
        END IF;
        SELECT directory.canonical_site_url(s.scheme,s.normalized_host,s.base_path) INTO canonical_url
            FROM directory.sites s WHERE s.normalized_host=target_host;
        IF canonical_url IS NOT NULL AND canonical_url<>target_url THEN
            RAISE EXCEPTION 'backup edge target is not canonical' USING ERRCODE='22023';
        END IF;
        INSERT INTO pg_temp.heyblog_restore_graph_edges VALUES(source_host,target_host);
        EXECUTE format($query$
            SELECT * FROM ag_catalog.cypher('directory_graph', $cypher$
                MATCH (s:SiteRef {normalized_host:%s}),(t:SiteRef {normalized_host:%s})
                CREATE (s)-[e:FRIEND_LINK {target_url:%s,status:%s,created_at_ms:%s,updated_at_ms:%s}]->(t)
                RETURN e
            $cypher$) AS (e ag_catalog.agtype)
        $query$,to_json(source_host)::text,to_json(target_host)::text,to_json(target_url)::text,to_json(edge_status)::text,created_ms,updated_ms);
    ELSE
        RAISE EXCEPTION 'unsupported backup graph dataset' USING ERRCODE='22023';
    END IF;
END;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.backup_restore_graph_finish()
RETURNS void
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
BEGIN
    IF NOT directory.backup_restore_is_active() THEN
        RAISE EXCEPTION 'database restore transaction required' USING ERRCODE='42501';
    END IF;
    IF (SELECT count(*) FROM directory.backup_graph_rows('vertices'))<>(SELECT count(*) FROM pg_temp.heyblog_restore_graph_vertices)
       OR (SELECT count(*) FROM directory.backup_graph_rows('edges'))<>(SELECT count(*) FROM pg_temp.heyblog_restore_graph_edges) THEN
        RAISE EXCEPTION 'restored backup graph coverage mismatch' USING ERRCODE='22023';
    END IF;
END;
$function$;
-- +goose StatementEnd

COMMENT ON FUNCTION directory.backup_graph_rows(text) IS 'Streams every logical SiteRef or FRIEND_LINK, including external isolates, non-visible sites and inactive edges, without physical AGE identifiers.';
COMMENT ON FUNCTION directory.backup_restore_graph_begin() IS 'Locks and resets the logical graph only within the protected database restore transaction; creates transaction-local input uniqueness guards.';
COMMENT ON FUNCTION directory.backup_restore_graph_row(text,jsonb) IS 'Restores one logical vertex or directed edge with canonical identity and exact status and millisecond timestamps; rejects duplicate input.';
COMMENT ON FUNCTION directory.backup_restore_graph_finish() IS 'Checks that the complete restored graph matches the accepted vertex and edge input coverage before commit.';
REVOKE ALL ON FUNCTION directory.backup_graph_rows(text),directory.backup_restore_graph_begin(),directory.backup_restore_graph_row(text,jsonb),directory.backup_restore_graph_finish() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION directory.backup_graph_rows(text),directory.backup_restore_graph_begin(),directory.backup_restore_graph_row(text,jsonb),directory.backup_restore_graph_finish() TO api_runtime;

-- +goose Down
DROP FUNCTION directory.backup_restore_graph_finish();
DROP FUNCTION directory.backup_restore_graph_row(text,jsonb);
DROP FUNCTION directory.backup_restore_graph_begin();
DROP FUNCTION directory.backup_graph_rows(text);

-- +goose Up

SET LOCAL check_function_bodies = false;

-- +goose StatementBegin

CREATE FUNCTION directory.backup_check_schema() RETURNS void
    LANGUAGE plpgsql STABLE
    SET search_path TO 'pg_catalog'
    AS $$
DECLARE signature text;
BEGIN
 SELECT md5(string_agg(n.nspname||'.'||c.relname||':'||a.attname||':'||a.atttypid::regtype::text||':'||a.attgenerated::text,',' ORDER BY n.nspname,c.relname,a.attnum)) INTO signature
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
 WHERE n.nspname IN ('identity','directory','content') AND c.relkind IN ('r','p');
 IF signature<>'87754795deb6c82c9f4be4017ce37bcf' THEN RAISE EXCEPTION 'unsupported backup schema' USING ERRCODE='22023'; END IF;
END $$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_columns() RETURNS TABLE(dataset text, column_name text, type_name text)
    LANGUAGE plpgsql STABLE SECURITY DEFINER
    SET search_path TO 'pg_catalog'
    AS $$
BEGIN
 PERFORM directory.backup_check_schema();
 RETURN QUERY SELECT t, a.attname::text,a.atttypid::regtype::text FROM unnest(directory.backup_tables()) t
 JOIN pg_attribute a ON a.attrelid=t::regclass AND a.attnum>0 AND NOT a.attisdropped ORDER BY t,a.attnum;
END;
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_encode_row(p_table text, p_row jsonb) RETURNS jsonb
    LANGUAGE plpgsql STABLE
    SET search_path TO 'pg_catalog'
    AS $$
DECLARE col record;
BEGIN
 IF NOT p_table=ANY(directory.backup_tables()) THEN RAISE EXCEPTION 'unsupported backup dataset'; END IF;
 FOR col IN SELECT attname,atttypid FROM pg_attribute WHERE attrelid=p_table::regclass AND attnum>0 AND NOT attisdropped LOOP
  IF p_row->col.attname <> 'null'::jsonb THEN
   IF col.atttypid='bytea'::regtype THEN
    p_row:=jsonb_set(p_row,ARRAY[col.attname],to_jsonb(encode((p_row->>col.attname)::bytea,'base64')));
   ELSIF col.atttypid='int8'::regtype THEN
    p_row:=jsonb_set(p_row,ARRAY[col.attname],to_jsonb(p_row->>col.attname));
   END IF;
  END IF;
 END LOOP;
 RETURN p_row;
END $$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_graph_rows(p_kind text) RETURNS SETOF jsonb
    LANGUAGE plpgsql STABLE SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'ag_catalog', 'directory'
    AS $_$
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
$_$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_map_history(p_value jsonb, p_source uuid, p_target uuid) RETURNS jsonb
    LANGUAGE plpgsql IMMUTABLE
    SET search_path TO 'pg_catalog'
    AS $$
DECLARE result jsonb; pair record;
BEGIN
 CASE jsonb_typeof(p_value)
 WHEN 'object' THEN
  result:='{}';
  FOR pair IN SELECT * FROM jsonb_each(p_value) LOOP
   IF pair.key=ANY(ARRAY['metadata','profile','settings']) THEN
    result:=result||jsonb_build_object(pair.key,pair.value);
   ELSIF pair.key=ANY(ARRAY['created_by','updated_by','reviewed_by','review_draft_updated_by','confirmed_by','granted_by','revoked_by','published_by','archived_by','changed_by','merged_by','actor_id','owner_id','user_id','submitter_user_id']) AND pair.value=to_jsonb(p_source::text) THEN
    result:=result||jsonb_build_object(pair.key,p_target::text);
   ELSE result:=result||jsonb_build_object(pair.key,directory.backup_map_history(pair.value,p_source,p_target)); END IF;
  END LOOP;
  RETURN result;
 WHEN 'array' THEN SELECT COALESCE(jsonb_agg(directory.backup_map_history(value,p_source,p_target)),'[]') INTO result FROM jsonb_array_elements(p_value); RETURN result;
 ELSE RETURN p_value;
 END CASE;
END $$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_restore_begin(p_admin uuid) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog'
    AS $$
DECLARE dataset text;
BEGIN
 IF NOT pg_try_advisory_xact_lock(728640312) THEN RAISE EXCEPTION 'restore running' USING ERRCODE='55P03'; END IF;
 PERFORM pg_advisory_xact_lock(74821953);
 FOREACH dataset IN ARRAY directory.backup_tables() LOOP
  IF dataset IN ('identity.users','identity.oauth_identities','identity.user_management_permissions') THEN
   -- Session checks read committed identities; writes remain blocked until restore completes.
   EXECUTE format('LOCK TABLE %s IN EXCLUSIVE MODE',dataset);
  ELSE
   EXECUTE format('LOCK TABLE %s IN ACCESS EXCLUSIVE MODE',dataset);
  END IF;
 END LOOP;
 LOCK TABLE directory_graph."_ag_label_vertex",directory_graph."_ag_label_edge",directory_graph."SiteRef",directory_graph."FRIEND_LINK" IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM directory.backup_target_issues(p_admin)) THEN RAISE EXCEPTION 'target not initialized' USING ERRCODE='55000'; END IF;
 CREATE TEMP TABLE heyblog_restore_guard(transaction_id bigint NOT NULL) ON COMMIT DROP;
 REVOKE ALL ON TABLE pg_temp.heyblog_restore_guard FROM PUBLIC,api_runtime;
 INSERT INTO pg_temp.heyblog_restore_guard VALUES(txid_current());
 SET CONSTRAINTS ALL DEFERRED;
 PERFORM directory.backup_restore_graph_begin();
 DELETE FROM directory.tag_identity_aliases;
 DELETE FROM directory.tag_cascades;
 DELETE FROM directory.tags;
 DELETE FROM directory.software_components;
END $$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_restore_finish() RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog'
    AS $$
BEGIN
 IF NOT directory.backup_restore_is_active() THEN RAISE EXCEPTION 'restore not active'; END IF;
 PERFORM directory.backup_restore_graph_finish();
 SET CONSTRAINTS ALL IMMEDIATE;
END $$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_restore_graph_begin() RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'ag_catalog', 'directory'
    AS $_$
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
$_$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_restore_graph_finish() RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'ag_catalog', 'directory'
    AS $$
BEGIN
    IF NOT directory.backup_restore_is_active() THEN
        RAISE EXCEPTION 'database restore transaction required' USING ERRCODE='42501';
    END IF;
    IF (SELECT count(*) FROM directory.backup_graph_rows('vertices'))<>(SELECT count(*) FROM pg_temp.heyblog_restore_graph_vertices)
       OR (SELECT count(*) FROM directory.backup_graph_rows('edges'))<>(SELECT count(*) FROM pg_temp.heyblog_restore_graph_edges) THEN
        RAISE EXCEPTION 'restored backup graph coverage mismatch' USING ERRCODE='22023';
    END IF;
    UPDATE runtime_cache.public_graph_revision SET revision = revision + 1 WHERE singleton;
END;
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_restore_graph_row(p_kind text, p_row jsonb) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'ag_catalog', 'directory'
    AS $_$
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
$_$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_restore_is_active() RETURNS boolean
    LANGUAGE plpgsql STABLE
    SET search_path TO 'pg_catalog'
    AS $$
DECLARE owner_id oid; guard_owner oid; active boolean;
BEGIN
 SELECT relowner INTO owner_id FROM pg_class WHERE oid='directory.sites'::regclass;
 IF current_user::regrole::oid<>owner_id THEN RETURN false; END IF;
 SELECT relowner INTO guard_owner FROM pg_class WHERE oid=to_regclass('pg_temp.heyblog_restore_guard');
 IF guard_owner IS DISTINCT FROM owner_id THEN RETURN false; END IF;
 EXECUTE 'SELECT transaction_id=txid_current() FROM pg_temp.heyblog_restore_guard' INTO active;
 RETURN COALESCE(active,false);
END $$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_restore_row(p_table text, p_row jsonb, p_source uuid, p_target uuid) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog'
    SET "TimeZone" TO 'UTC'
    AS $_$
DECLARE col record; columns_ text; original jsonb; written jsonb; field text;
BEGIN
 IF NOT directory.backup_restore_is_active() OR NOT p_table=ANY(directory.backup_tables()) THEN RAISE EXCEPTION 'restore not active'; END IF;
 IF p_table='identity.users' AND (p_row->>'role'='SYS_ADMIN' OR p_row->>'id'=p_source::text OR p_row->>'id'=p_target::text) THEN RAISE EXCEPTION 'excluded administrator in backup'; END IF;
 IF p_table=ANY(ARRAY['identity.oauth_identities','identity.email_verification_codes','identity.password_reset_tokens','identity.user_management_permissions']) AND p_row->>'user_id'=p_source::text THEN RAISE EXCEPTION 'excluded administrator authentication in backup'; END IF;
 IF (SELECT array_agg(key ORDER BY key COLLATE "C") FROM jsonb_object_keys(p_row) key) IS DISTINCT FROM
    (SELECT array_agg(attname::text ORDER BY attname::text COLLATE "C") FROM pg_attribute WHERE attrelid=p_table::regclass AND attnum>0 AND NOT attisdropped) THEN RAISE EXCEPTION 'backup columns mismatch'; END IF;
 FOR col IN SELECT a.attname FROM pg_constraint c JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=ANY(c.conkey)
  WHERE c.conrelid=p_table::regclass AND c.confrelid='identity.users'::regclass LOOP
  IF p_row->>col.attname=p_source::text THEN p_row:=jsonb_set(p_row,ARRAY[col.attname],to_jsonb(p_target::text)); END IF;
 END LOOP;
 IF p_table=ANY(ARRAY['directory.site_audits','directory.tag_identity_aliases','directory.tag_assignment_archive']) THEN
  FOR col IN SELECT attname FROM pg_attribute WHERE attrelid=p_table::regclass AND atttypid='jsonb'::regtype AND attnum>0 AND NOT attisdropped LOOP
   p_row:=jsonb_set(p_row,ARRAY[col.attname],directory.backup_map_history(p_row->col.attname,p_source,p_target));
  END LOOP;
 END IF;
 IF p_table='directory.site_origins' AND p_row->'metadata'->>'channel'=ANY(ARRAY['ACCOUNT_SUBMISSION','OWNER_FRIEND_LINK']) THEN
  IF p_row->'metadata'->>'user_id'=p_source::text THEN
   p_row:=jsonb_set(p_row,'{metadata,user_id}',to_jsonb(p_target::text));
  END IF;
  IF jsonb_typeof(p_row->'metadata'->'recommendations')='array' THEN
   p_row:=jsonb_set(p_row,'{metadata,recommendations}',(SELECT COALESCE(jsonb_agg(CASE WHEN item->>'user_id'=p_source::text THEN jsonb_set(item,'{user_id}',to_jsonb(p_target::text)) ELSE item END),'[]') FROM jsonb_array_elements(p_row->'metadata'->'recommendations') item));
  END IF;
 END IF;
 original:=p_row;
 FOR col IN SELECT attname FROM pg_attribute WHERE attrelid=p_table::regclass AND atttypid='bytea'::regtype AND attnum>0 AND NOT attisdropped LOOP
  IF p_row->col.attname<>'null'::jsonb THEN p_row:=jsonb_set(p_row,ARRAY[col.attname],to_jsonb(decode(p_row->>col.attname,'base64')::text)); END IF;
 END LOOP;
 SELECT string_agg(quote_ident(attname),',' ORDER BY attnum) INTO columns_ FROM pg_attribute WHERE attrelid=p_table::regclass AND attnum>0 AND NOT attisdropped AND attgenerated='';
 EXECUTE format('INSERT INTO %s (%s) SELECT %s FROM jsonb_populate_record(NULL::%s,$1) RETURNING to_jsonb(%s.*)',p_table,columns_,columns_,p_table,p_table) INTO written USING p_row;
 IF directory.backup_encode_row(p_table,written)<>original THEN RAISE EXCEPTION 'restored row differs from backup'; END IF;
END $_$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_seed_fingerprint() RETURNS text
    LANGUAGE plpgsql STABLE SECURITY DEFINER
    SET search_path TO 'pg_catalog'
    AS $_$
DECLARE ids jsonb; dataset text; records jsonb; result jsonb:='{}';
BEGIN
 SELECT COALESCE(jsonb_object_agg(id,label),'{}') INTO ids FROM (
  SELECT alias_id::text id,'alias:'||system_key label FROM directory.tag_identity_aliases WHERE alias_kind='TAG' AND system_key IS NOT NULL
  UNION ALL SELECT t.id::text,'tag:'||t.normalized_name FROM directory.tags t
  UNION ALL SELECT c.id::text,'cascade:'||c.scope||':'||c.taxonomy_key FROM directory.tag_cascades c
  UNION ALL SELECT id::text,'component:'||normalized_name FROM directory.software_components
 ) identities;
 FOREACH dataset IN ARRAY ARRAY['directory.tags','directory.tag_cascades','directory.tag_identity_aliases','directory.software_components'] LOOP
  EXECUTE format('SELECT COALESCE(jsonb_agg(row ORDER BY row::text),''[]''::jsonb) FROM (SELECT directory.backup_seed_scrub(to_jsonb(t),$1) row FROM %s t) s',dataset) INTO records USING ids;
  result:=result||jsonb_build_object(dataset,records);
 END LOOP;
 RETURN md5(result::text);
END $_$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_seed_scrub(p_value jsonb, p_ids jsonb) RETURNS jsonb
    LANGUAGE plpgsql IMMUTABLE
    SET search_path TO 'pg_catalog'
    AS $$
DECLARE result jsonb; pair record;
BEGIN
 CASE jsonb_typeof(p_value)
 WHEN 'object' THEN
  result:='{}';
  FOR pair IN SELECT * FROM jsonb_each(p_value) LOOP
   IF pair.key NOT IN ('created_at','updated_at','archived_at') THEN result:=result||jsonb_build_object(pair.key,directory.backup_seed_scrub(pair.value,p_ids)); END IF;
  END LOOP;
  RETURN result;
 WHEN 'array' THEN
  SELECT COALESCE(jsonb_agg(directory.backup_seed_scrub(value,p_ids)),'[]') INTO result FROM jsonb_array_elements(p_value);
  RETURN result;
 WHEN 'string' THEN RETURN COALESCE(p_ids->(p_value#>>'{}'),p_value);
 ELSE RETURN p_value;
 END CASE;
END $$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_table_rows(p_table text, p_admin uuid) RETURNS SETOF jsonb
    LANGUAGE plpgsql STABLE SECURITY DEFINER
    SET search_path TO 'pg_catalog'
    SET "TimeZone" TO 'UTC'
    AS $_$
DECLARE filter text:='';
BEGIN
 PERFORM directory.backup_check_schema();
 IF NOT p_table=ANY(directory.backup_tables()) THEN RAISE EXCEPTION 'unsupported backup dataset'; END IF;
 IF p_table='identity.users' THEN filter:=' WHERE id<>$1';
 ELSIF p_table=ANY(ARRAY['identity.oauth_identities','identity.email_verification_codes','identity.password_reset_tokens','identity.user_management_permissions']) THEN filter:=' WHERE user_id<>$1'; END IF;
 RETURN QUERY EXECUTE format('SELECT directory.backup_encode_row(%L,to_jsonb(t)) FROM %s t%s ORDER BY to_jsonb(t)::text',p_table,p_table,filter) USING p_admin;
END $_$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_tables() RETURNS text[]
    LANGUAGE sql IMMUTABLE
    AS $$
 SELECT ARRAY[
 'identity.users','identity.oauth_identities','identity.email_verification_codes','identity.password_reset_tokens','identity.user_management_permissions',
 'identity.api_clients','identity.api_client_scopes','identity.api_keys',
 'directory.tags','directory.tag_cascades','directory.tag_identity_aliases','directory.tag_assignment_archive',
 'directory.software_components','directory.software_component_dependencies','directory.site_sources',
 'directory.sites','directory.site_feeds','directory.site_icons','directory.site_resources','directory.site_software_components','directory.site_origins','directory.site_tags',
 'directory.site_claims','directory.site_ownerships','directory.site_ownership_events',
 'directory.site_audits','directory.owner_friend_link_requests','directory.site_metrics','directory.site_metric_events',
 'content.articles','content.article_tags','content.announcements','content.announcement_revisions']::text[];
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION directory.backup_target_issues(p_admin uuid) RETURNS SETOF text
    LANGUAGE plpgsql STABLE SECURITY DEFINER
    SET search_path TO 'pg_catalog'
    AS $$
DECLARE dataset text; occupied boolean;
BEGIN
 IF (SELECT count(*) FROM identity.users)<>1 OR NOT EXISTS(SELECT 1 FROM identity.users WHERE id=p_admin AND role='SYS_ADMIN' AND access_status='ACTIVE' AND deleted_at IS NULL) THEN
  RETURN NEXT 'target_admin_invalid';
 END IF;
 FOREACH dataset IN ARRAY directory.backup_tables() LOOP
  IF dataset=ANY(ARRAY['identity.users','identity.oauth_identities','identity.email_verification_codes','identity.password_reset_tokens','identity.user_management_permissions','directory.tags','directory.tag_cascades','directory.tag_identity_aliases','directory.software_components']) THEN CONTINUE; END IF;
  EXECUTE format('SELECT EXISTS(SELECT 1 FROM %s)',dataset) INTO occupied;
  IF occupied THEN RETURN NEXT 'target_not_empty'; RETURN; END IF;
 END LOOP;
 IF directory.backup_seed_fingerprint()<>'5f865e286d7181374a1d15c0efc64abc' THEN RETURN NEXT 'target_seed_changed'; END IF;
 IF EXISTS(SELECT 1 FROM directory.backup_graph_rows('vertices')) OR EXISTS(SELECT 1 FROM directory.backup_graph_rows('edges')) THEN RETURN NEXT 'target_graph_not_empty'; END IF;
END $$;

-- +goose StatementEnd

COMMENT ON FUNCTION directory.backup_check_schema() IS 'Rejects schema drift, including new datasets, columns or types that require a new backup contract version.';

COMMENT ON FUNCTION directory.backup_columns() IS 'Exposes the exact supported backup row field and PostgreSQL type inventory.';

COMMENT ON FUNCTION directory.backup_encode_row(p_table text, p_row jsonb) IS 'Encodes binary fields as Base64 and bigint fields as lossless decimal strings.';

COMMENT ON FUNCTION directory.backup_graph_rows(p_kind text) IS 'Streams every logical SiteRef or FRIEND_LINK, including external isolates, non-visible sites and inactive edges, without physical AGE identifiers.';

COMMENT ON FUNCTION directory.backup_map_history(p_value jsonb, p_source uuid, p_target uuid) IS 'Maps explicitly named actor references inside archival JSON without rewriting free-text values.';

COMMENT ON FUNCTION directory.backup_restore_begin(p_admin uuid) IS 'Locks all application datasets and validates initialization before opening the protected atomic restore scope.';

COMMENT ON FUNCTION directory.backup_restore_finish() IS 'Validates complete graph and deferred relational constraints before committing a restore.';

COMMENT ON FUNCTION directory.backup_restore_graph_begin() IS 'Locks and resets the logical graph only within the protected database restore transaction; creates transaction-local input uniqueness guards.';

COMMENT ON FUNCTION directory.backup_restore_graph_finish() IS 'Checks that the complete restored graph matches the accepted vertex and edge input coverage before commit.';

COMMENT ON FUNCTION directory.backup_restore_graph_row(p_kind text, p_row jsonb) IS 'Restores one logical vertex or directed edge with canonical identity and exact status and millisecond timestamps; rejects duplicate input.';

COMMENT ON FUNCTION directory.backup_restore_is_active() IS 'Checks migrator ownership and a protected transaction-local restore guard; runtime-set session settings cannot authorize restoration.';

COMMENT ON FUNCTION directory.backup_restore_row(p_table text, p_row jsonb, p_source uuid, p_target uuid) IS 'Restores one allowlisted complete row with actor remapping and exact post-insert content verification.';

COMMENT ON FUNCTION directory.backup_seed_fingerprint() IS 'Fingerprints complete logical initialization seed content and relationships.';

COMMENT ON FUNCTION directory.backup_seed_scrub(p_value jsonb, p_ids jsonb) IS 'Normalizes generated seed identifiers and creation timestamps for initialization comparison.';

COMMENT ON FUNCTION directory.backup_table_rows(p_table text, p_admin uuid) IS 'Reads all dataset states while excluding only the source administrator and self-owned authentication records.';

COMMENT ON FUNCTION directory.backup_tables() IS 'Protocol-one schema-three exhaustive application backup dataset allowlist in dependency order.';

COMMENT ON FUNCTION directory.backup_target_issues(p_admin uuid) IS 'Rejects populated or modified targets while preserving the retained administrator authentication records.';

REVOKE ALL ON FUNCTION directory.backup_check_schema() FROM PUBLIC;

REVOKE ALL ON FUNCTION directory.backup_columns() FROM PUBLIC;
GRANT ALL ON FUNCTION directory.backup_columns() TO api_runtime;

REVOKE ALL ON FUNCTION directory.backup_encode_row(p_table text, p_row jsonb) FROM PUBLIC;

REVOKE ALL ON FUNCTION directory.backup_graph_rows(p_kind text) FROM PUBLIC;
GRANT ALL ON FUNCTION directory.backup_graph_rows(p_kind text) TO api_runtime;

REVOKE ALL ON FUNCTION directory.backup_map_history(p_value jsonb, p_source uuid, p_target uuid) FROM PUBLIC;

REVOKE ALL ON FUNCTION directory.backup_restore_begin(p_admin uuid) FROM PUBLIC;
GRANT ALL ON FUNCTION directory.backup_restore_begin(p_admin uuid) TO api_runtime;

REVOKE ALL ON FUNCTION directory.backup_restore_finish() FROM PUBLIC;
GRANT ALL ON FUNCTION directory.backup_restore_finish() TO api_runtime;

REVOKE ALL ON FUNCTION directory.backup_restore_graph_begin() FROM PUBLIC;
GRANT ALL ON FUNCTION directory.backup_restore_graph_begin() TO api_runtime;

REVOKE ALL ON FUNCTION directory.backup_restore_graph_finish() FROM PUBLIC;
GRANT ALL ON FUNCTION directory.backup_restore_graph_finish() TO api_runtime;

REVOKE ALL ON FUNCTION directory.backup_restore_graph_row(p_kind text, p_row jsonb) FROM PUBLIC;
GRANT ALL ON FUNCTION directory.backup_restore_graph_row(p_kind text, p_row jsonb) TO api_runtime;

REVOKE ALL ON FUNCTION directory.backup_restore_is_active() FROM PUBLIC;
GRANT ALL ON FUNCTION directory.backup_restore_is_active() TO api_runtime;

REVOKE ALL ON FUNCTION directory.backup_restore_row(p_table text, p_row jsonb, p_source uuid, p_target uuid) FROM PUBLIC;
GRANT ALL ON FUNCTION directory.backup_restore_row(p_table text, p_row jsonb, p_source uuid, p_target uuid) TO api_runtime;

REVOKE ALL ON FUNCTION directory.backup_seed_fingerprint() FROM PUBLIC;

REVOKE ALL ON FUNCTION directory.backup_seed_scrub(p_value jsonb, p_ids jsonb) FROM PUBLIC;

REVOKE ALL ON FUNCTION directory.backup_table_rows(p_table text, p_admin uuid) FROM PUBLIC;
GRANT ALL ON FUNCTION directory.backup_table_rows(p_table text, p_admin uuid) TO api_runtime;

REVOKE ALL ON FUNCTION directory.backup_tables() FROM PUBLIC;

REVOKE ALL ON FUNCTION directory.backup_target_issues(p_admin uuid) FROM PUBLIC;
GRANT ALL ON FUNCTION directory.backup_target_issues(p_admin uuid) TO api_runtime;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The consolidated baseline is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd

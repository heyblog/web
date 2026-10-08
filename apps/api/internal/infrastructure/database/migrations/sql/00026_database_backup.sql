-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION directory.backup_tables() RETURNS text[] LANGUAGE sql IMMUTABLE AS $$
 SELECT ARRAY[
 'identity.users','identity.oauth_identities','identity.email_verification_codes','identity.password_reset_tokens','identity.user_management_permissions',
 'identity.api_clients','identity.api_client_scopes','identity.api_keys',
 'directory.tags','directory.tag_labels','directory.tag_cascades','directory.tag_identity_aliases','directory.tag_slug_aliases','directory.tag_assignment_archive',
 'directory.software_components','directory.software_component_dependencies','directory.site_sources',
 'directory.sites','directory.site_feeds','directory.site_icons','directory.site_resources','directory.site_software_components','directory.site_origins','directory.site_tags',
 'directory.site_claims','directory.site_ownerships','directory.site_ownership_events',
 'directory.site_audits','directory.owner_friend_link_requests','directory.slug_generation_cache','directory.slug_generation_jobs','directory.site_metrics','directory.site_metric_events',
 'content.articles','content.article_tags','content.announcements','content.announcement_revisions','content.system_ai_settings']::text[];
$$;

CREATE FUNCTION directory.backup_check_schema() RETURNS void LANGUAGE plpgsql STABLE SET search_path=pg_catalog AS $$
DECLARE signature text;
BEGIN
 SELECT md5(string_agg(n.nspname||'.'||c.relname||':'||a.attname||':'||a.atttypid::regtype::text||':'||a.attgenerated::text,',' ORDER BY n.nspname,c.relname,a.attnum)) INTO signature
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
 WHERE n.nspname IN ('identity','directory','content') AND c.relkind IN ('r','p');
 IF signature<>'ca4ae5593064ddf45f111eef9b9c4fff' THEN RAISE EXCEPTION 'unsupported backup schema' USING ERRCODE='22023'; END IF;
END $$;

CREATE FUNCTION directory.backup_encode_row(p_table text,p_row jsonb) RETURNS jsonb
LANGUAGE plpgsql STABLE SET search_path=pg_catalog AS $$
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

CREATE FUNCTION directory.backup_table_rows(p_table text,p_admin uuid) RETURNS SETOF jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog SET timezone='UTC' AS $$
DECLARE filter text:='';
BEGIN
 PERFORM directory.backup_check_schema();
 IF NOT p_table=ANY(directory.backup_tables()) THEN RAISE EXCEPTION 'unsupported backup dataset'; END IF;
 IF p_table='identity.users' THEN filter:=' WHERE id<>$1';
 ELSIF p_table=ANY(ARRAY['identity.oauth_identities','identity.email_verification_codes','identity.password_reset_tokens','identity.user_management_permissions']) THEN filter:=' WHERE user_id<>$1'; END IF;
 RETURN QUERY EXECUTE format('SELECT directory.backup_encode_row(%L,to_jsonb(t)) FROM %s t%s ORDER BY to_jsonb(t)::text',p_table,p_table,filter) USING p_admin;
END $$;

CREATE FUNCTION directory.backup_columns() RETURNS TABLE(dataset text,column_name text,type_name text)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM directory.backup_check_schema();
 RETURN QUERY SELECT t, a.attname::text,a.atttypid::regtype::text FROM unnest(directory.backup_tables()) t
 JOIN pg_attribute a ON a.attrelid=t::regclass AND a.attnum>0 AND NOT a.attisdropped ORDER BY t,a.attnum;
END;
$$;

CREATE FUNCTION directory.backup_restore_is_active() RETURNS boolean
LANGUAGE plpgsql STABLE SET search_path=pg_catalog AS $$
DECLARE owner_id oid; guard_owner oid; active boolean;
BEGIN
 SELECT relowner INTO owner_id FROM pg_class WHERE oid='directory.sites'::regclass;
 IF current_user::regrole::oid<>owner_id THEN RETURN false; END IF;
 SELECT relowner INTO guard_owner FROM pg_class WHERE oid=to_regclass('pg_temp.heyblog_restore_guard');
 IF guard_owner IS DISTINCT FROM owner_id THEN RETURN false; END IF;
 EXECUTE 'SELECT transaction_id=txid_current() FROM pg_temp.heyblog_restore_guard' INTO active;
 RETURN COALESCE(active,false);
END $$;

CREATE FUNCTION directory.backup_seed_scrub(p_value jsonb,p_ids jsonb) RETURNS jsonb
LANGUAGE plpgsql IMMUTABLE SET search_path=pg_catalog AS $$
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

CREATE FUNCTION directory.backup_seed_fingerprint() RETURNS text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ids jsonb; dataset text; records jsonb; result jsonb:='{}';
BEGIN
 SELECT COALESCE(jsonb_object_agg(id,label),'{}') INTO ids FROM (
  SELECT alias_id::text id,'alias:'||system_key label FROM directory.tag_identity_aliases
  UNION ALL SELECT t.id::text,'tag:'||l.normalized_name FROM directory.tags t JOIN directory.tag_labels l ON l.id=t.default_label_id
  UNION ALL SELECT c.id::text,'cascade:'||c.scope||':'||c.taxonomy_key FROM directory.tag_cascades c
  UNION ALL SELECT id::text,'component:'||normalized_name FROM directory.software_components
 ) identities;
 FOREACH dataset IN ARRAY ARRAY['directory.tags','directory.tag_labels','directory.tag_cascades','directory.tag_identity_aliases','directory.tag_slug_aliases','directory.software_components'] LOOP
  EXECUTE format('SELECT COALESCE(jsonb_agg(row ORDER BY row::text),''[]''::jsonb) FROM (SELECT directory.backup_seed_scrub(to_jsonb(t),$1) row FROM %s t) s',dataset) INTO records USING ids;
  result:=result||jsonb_build_object(dataset,records);
 END LOOP;
 RETURN md5(result::text);
END $$;

CREATE FUNCTION directory.backup_target_issues(p_admin uuid) RETURNS SETOF text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE dataset text; occupied boolean;
BEGIN
 IF (SELECT count(*) FROM identity.users)<>1 OR NOT EXISTS(SELECT 1 FROM identity.users WHERE id=p_admin AND role='SYS_ADMIN' AND access_status='ACTIVE' AND deleted_at IS NULL) THEN
  RETURN NEXT 'target_admin_invalid';
 END IF;
 FOREACH dataset IN ARRAY directory.backup_tables() LOOP
  IF dataset=ANY(ARRAY['identity.users','identity.oauth_identities','identity.email_verification_codes','identity.password_reset_tokens','identity.user_management_permissions','directory.tags','directory.tag_labels','directory.tag_cascades','directory.tag_identity_aliases','directory.tag_slug_aliases','directory.software_components']) THEN CONTINUE; END IF;
  EXECUTE format('SELECT EXISTS(SELECT 1 FROM %s)',dataset) INTO occupied;
  IF occupied THEN RETURN NEXT 'target_not_empty'; RETURN; END IF;
 END LOOP;
 IF directory.backup_seed_fingerprint()<>'1e66722cceb9f044a04fbadc0a132d43' THEN RETURN NEXT 'target_seed_changed'; END IF;
 IF EXISTS(SELECT 1 FROM directory.backup_graph_rows('vertices')) OR EXISTS(SELECT 1 FROM directory.backup_graph_rows('edges')) THEN RETURN NEXT 'target_graph_not_empty'; END IF;
END $$;

CREATE FUNCTION directory.backup_restore_begin(p_admin uuid) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE dataset text;
BEGIN
 IF NOT pg_try_advisory_xact_lock(728640312) THEN RAISE EXCEPTION 'restore running' USING ERRCODE='55P03'; END IF;
 PERFORM pg_advisory_xact_lock(74821953);
 FOREACH dataset IN ARRAY directory.backup_tables() LOOP EXECUTE format('LOCK TABLE %s IN ACCESS EXCLUSIVE MODE',dataset); END LOOP;
 LOCK TABLE directory_graph."_ag_label_vertex",directory_graph."_ag_label_edge",directory_graph."SiteRef",directory_graph."FRIEND_LINK" IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM directory.backup_target_issues(p_admin)) THEN RAISE EXCEPTION 'target not initialized' USING ERRCODE='55000'; END IF;
 CREATE TEMP TABLE heyblog_restore_guard(transaction_id bigint NOT NULL) ON COMMIT DROP;
 REVOKE ALL ON TABLE pg_temp.heyblog_restore_guard FROM PUBLIC,api_runtime;
 INSERT INTO pg_temp.heyblog_restore_guard VALUES(txid_current());
 SET CONSTRAINTS ALL DEFERRED;
 PERFORM directory.backup_restore_graph_begin();
 DELETE FROM directory.tag_slug_aliases;
 DELETE FROM directory.tag_identity_aliases;
 DELETE FROM directory.tag_cascades;
 DELETE FROM directory.tag_labels;
 DELETE FROM directory.tags;
 DELETE FROM directory.software_components;
END $$;

CREATE FUNCTION directory.backup_map_history(p_value jsonb,p_source uuid,p_target uuid) RETURNS jsonb
LANGUAGE plpgsql IMMUTABLE SET search_path=pg_catalog AS $$
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

CREATE FUNCTION directory.backup_restore_row(p_table text,p_row jsonb,p_source uuid,p_target uuid) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog SET timezone='UTC' AS $$
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
END $$;

CREATE FUNCTION directory.backup_restore_finish() RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NOT directory.backup_restore_is_active() THEN RAISE EXCEPTION 'restore not active'; END IF;
 PERFORM directory.backup_restore_graph_finish();
 SET CONSTRAINTS ALL IMMEDIATE;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.ensure_site_cascade_scope() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE OR REPLACE FUNCTION content.ensure_article_cascade_scope() RETURNS trigger LANGUAGE plpgsql AS $$
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


-- Self-referential chains may be serialized in either order; final FK checking
-- remains mandatory and ordinary operations retain immediate checking.
-- +goose StatementBegin
DO $$ DECLARE c record; BEGIN
 FOR c IN SELECT conrelid::regclass relation,conname FROM pg_constraint WHERE contype='f' AND conrelid=confrelid AND conrelid IN ('identity.api_keys'::regclass,'directory.tag_cascades'::regclass) LOOP
  EXECUTE format('ALTER TABLE %s ALTER CONSTRAINT %I DEFERRABLE INITIALLY IMMEDIATE',c.relation,c.conname);
 END LOOP;
END $$;
-- +goose StatementEnd

COMMENT ON FUNCTION directory.backup_tables() IS 'Protocol-one schema-two exhaustive application backup dataset allowlist in dependency order.';
COMMENT ON FUNCTION directory.backup_check_schema() IS 'Rejects schema drift, including new datasets, columns or types that require a new backup contract version.';
COMMENT ON FUNCTION directory.backup_encode_row(text,jsonb) IS 'Encodes binary fields as Base64 and bigint fields as lossless decimal strings.';
COMMENT ON FUNCTION directory.backup_table_rows(text,uuid) IS 'Reads all dataset states while excluding only the source administrator and self-owned authentication records.';
COMMENT ON FUNCTION directory.backup_columns() IS 'Exposes the exact supported backup row field and PostgreSQL type inventory.';
COMMENT ON FUNCTION directory.backup_restore_is_active() IS 'Checks migrator ownership and a protected transaction-local restore guard; runtime-set session settings cannot authorize restoration.';
COMMENT ON FUNCTION directory.backup_seed_scrub(jsonb,jsonb) IS 'Normalizes generated seed identifiers and creation timestamps for initialization comparison.';
COMMENT ON FUNCTION directory.backup_seed_fingerprint() IS 'Fingerprints complete logical initialization seed content and relationships.';
COMMENT ON FUNCTION directory.backup_target_issues(uuid) IS 'Rejects populated or modified targets while preserving the retained administrator authentication records.';
COMMENT ON FUNCTION directory.backup_restore_begin(uuid) IS 'Locks all application datasets and validates initialization before opening the protected atomic restore scope.';
COMMENT ON FUNCTION directory.backup_map_history(jsonb,uuid,uuid) IS 'Maps explicitly named actor references inside archival JSON without rewriting free-text values.';
COMMENT ON FUNCTION directory.backup_restore_row(text,jsonb,uuid,uuid) IS 'Restores one allowlisted complete row with actor remapping and exact post-insert content verification.';
COMMENT ON FUNCTION directory.backup_restore_finish() IS 'Validates complete graph and deferred relational constraints before committing a restore.';

REVOKE ALL ON FUNCTION directory.backup_tables(),directory.backup_encode_row(text,jsonb),directory.backup_table_rows(text,uuid),directory.backup_columns(),directory.backup_restore_is_active(),directory.backup_seed_scrub(jsonb,jsonb),directory.backup_seed_fingerprint(),directory.backup_target_issues(uuid),directory.backup_restore_begin(uuid),directory.backup_map_history(jsonb,uuid,uuid),directory.backup_restore_row(text,jsonb,uuid,uuid),directory.backup_restore_finish() FROM PUBLIC;
REVOKE ALL ON FUNCTION directory.backup_check_schema() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION directory.backup_table_rows(text,uuid),directory.backup_columns(),directory.backup_target_issues(uuid),directory.backup_restore_begin(uuid),directory.backup_restore_row(text,jsonb,uuid,uuid),directory.backup_restore_finish(),directory.backup_restore_is_active() TO api_runtime;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.validate_owner_audit_submission()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    owner_row directory.site_ownerships;
    current_url text;
    proposed_url text;
BEGIN
    IF directory.backup_restore_is_active() THEN RETURN NEW; END IF;
    IF NEW.source_channel NOT IN ('OWNER_UPDATE', 'OWNER_FRIEND_LINK') THEN RETURN NEW; END IF;
    SELECT scheme || '://' || normalized_host || base_path INTO current_url
      FROM directory.sites WHERE id = NEW.source_site_id AND visibility <> 'REMOVED' FOR UPDATE;
    SELECT * INTO owner_row FROM directory.site_ownerships WHERE id = NEW.ownership_id FOR UPDATE;
    IF current_url IS NULL OR owner_row.id IS NULL OR owner_row.site_id <> NEW.source_site_id
       OR owner_row.user_id IS DISTINCT FROM NEW.submitter_user_id OR owner_row.address <> current_url THEN
        RAISE EXCEPTION 'active site ownership is required' USING ERRCODE = 'P0001';
    END IF;
    IF NEW.source_channel = 'OWNER_UPDATE' THEN
        IF NEW.action <> 'UPDATE' OR NEW.site_id <> NEW.source_site_id THEN
            RAISE EXCEPTION 'invalid owner update target';
        END IF;
        proposed_url := (NEW.proposed_snapshot ->> 'scheme') || '://' || (NEW.proposed_snapshot ->> 'normalized_host') || (NEW.proposed_snapshot ->> 'base_path');
        IF proposed_url <> current_url AND NOT EXISTS (
            SELECT 1 FROM directory.site_claims WHERE site_id = NEW.site_id AND user_id = NEW.submitter_user_id
             AND address = proposed_url AND status = 'VERIFIED' AND consumed_at IS NULL
             AND created_at >= owner_row.created_at
        ) THEN RAISE EXCEPTION 'new site address verification is required'; END IF;
    ELSIF NEW.action <> 'CREATE' THEN
        RAISE EXCEPTION 'friend link submission must create a site';
    END IF;
    RETURN NEW;
END;
$$;

-- +goose StatementEnd
COMMENT ON FUNCTION directory.validate_owner_audit_submission() IS 'Validates live owner authority for submissions; the protected atomic restore scope preserves historical ownership generations.';

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Migration 00026 is forward-only; restore the pre-upgrade database backup.'; END $$;
-- +goose StatementEnd

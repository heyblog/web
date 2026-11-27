-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_check_schema() RETURNS void
    LANGUAGE plpgsql STABLE
    SET search_path TO 'pg_catalog'
    AS $$
DECLARE signature text;
BEGIN
    SELECT md5(string_agg(n.nspname||'.'||c.relname||':'||a.attname||':'||a.atttypid::regtype::text||':'||a.attgenerated::text,',' ORDER BY n.nspname,c.relname,a.attnum))
      INTO signature
      FROM pg_class c
      JOIN pg_namespace n ON n.oid=c.relnamespace
      JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
     WHERE n.nspname IN ('identity','directory','content') AND c.relkind IN ('r','p');
    IF signature IS DISTINCT FROM '9d9239ee3c6ab00298f528d5a4d76b7e' THEN
        RAISE EXCEPTION 'unsupported backup schema' USING ERRCODE='22023';
    END IF;
END;
$$;
-- +goose StatementEnd

SELECT directory.backup_check_schema();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_tables() RETURNS text[]
    LANGUAGE sql IMMUTABLE
    AS $$
 SELECT ARRAY[
 'identity.users','identity.oauth_identities','identity.email_verification_codes','identity.password_reset_tokens','identity.user_management_permissions',
 'identity.api_clients','identity.api_client_scopes','identity.api_keys',
 'directory.tags','directory.tag_cascades','directory.tag_identity_aliases','directory.tag_assignment_archive',
 'directory.software_components','directory.software_component_dependencies','directory.site_sources',
 'directory.sites','directory.site_feeds','directory.site_icons','directory.site_resources','directory.site_software_components','directory.site_origins','directory.site_tags',
 'directory.site_claims','directory.site_claim_attachments','directory.site_ownerships','directory.site_ownership_events',
 'directory.site_audits','directory.site_annotation_history','directory.owner_friend_link_requests','directory.site_metrics','directory.site_metric_events',
 'content.articles','content.article_tags','content.announcements','content.announcement_revisions']::text[];
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_restore_row(p_table text, p_row jsonb, p_source uuid, p_target uuid) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog'
    SET "TimeZone" TO 'UTC'
    AS $_$
DECLARE col record; columns_ text; original jsonb; written jsonb;
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
 IF p_table=ANY(ARRAY['directory.site_audits','directory.site_annotation_history','directory.tag_identity_aliases','directory.tag_assignment_archive']) THEN
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
END;
$_$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.backup_restore_finish(p_schema_version integer) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog'
    AS $$
BEGIN
    IF p_schema_version IS NULL OR p_schema_version NOT BETWEEN 3 AND 7 THEN
        RAISE EXCEPTION 'unsupported backup schema' USING ERRCODE='22023';
    END IF;

    -- The existing finish verifies the restore guard and flushes deferred FK events.
    PERFORM directory.backup_restore_finish();
    IF p_schema_version < 7 THEN
        -- Match migration 17: only an approved CREATE or UPDATE establishes BLOG.
        ALTER TABLE directory.sites DISABLE TRIGGER sites_touch_site;
        UPDATE directory.sites AS site
           SET blog_status = 'BLOG'
         WHERE site.blog_status = 'UNREVIEWED'
           AND EXISTS (
               SELECT 1 FROM directory.site_audits AS audit
                WHERE audit.site_id = site.id AND audit.status = 'APPROVED'
                  AND audit.action IN ('CREATE', 'UPDATE')
           );
        ALTER TABLE directory.sites ENABLE TRIGGER sites_touch_site;
    END IF;
END;
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION directory.backup_restore_finish(integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION directory.backup_restore_finish(integer) TO api_runtime;

COMMENT ON FUNCTION directory.backup_check_schema() IS 'Checks the exact schema including site classification and annotation history.';
COMMENT ON FUNCTION directory.backup_tables() IS 'Protocol-one schema-seven exhaustive application backup dataset allowlist in dependency order.';
COMMENT ON FUNCTION directory.backup_restore_finish(integer) IS 'Validates the guarded restore and backfills blog status for backups that predate site annotation.';

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Cannot downgrade site annotation backups; restore the previous database backup.'; END $$;
-- +goose StatementEnd

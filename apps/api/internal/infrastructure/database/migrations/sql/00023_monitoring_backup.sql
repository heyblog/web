-- +goose Up


-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_tables() RETURNS text[]
LANGUAGE sql IMMUTABLE AS $$
 SELECT ARRAY[
 'identity.users','identity.oauth_identities','identity.email_verification_codes','identity.password_reset_tokens','identity.user_management_permissions',
 'identity.api_clients','identity.api_client_scopes','identity.api_keys',
 'directory.tags','directory.tag_cascades','directory.tag_identity_aliases','directory.tag_assignment_archive',
 'directory.software_components','directory.software_component_dependencies','directory.site_sources',
 'directory.sites','directory.site_feeds','directory.site_icons','directory.site_resources','directory.site_software_components','directory.site_origins','directory.site_tags',
 'directory.site_claims','directory.site_claim_attachments','directory.site_ownerships','directory.site_ownership_events',
 'directory.site_audits','directory.site_annotation_history','directory.owner_friend_link_requests','directory.site_metrics','directory.site_metric_events',
 'content.articles','content.article_tags','content.announcements','content.announcement_revisions',
 'directory.site_monitoring_rounds',
 'operations.request_configs','operations.task_settings','operations.workers','operations.schedules','operations.runs','operations.targets',
 'operations.signing_policy','operations.signing_keys','operations.signing_events','operations.request_attempts',
 'operations.feed_documents','operations.feed_items','operations.route_state',
 'operations.task_events','operations.task_idempotency','operations.platform_backoff','operations.request_statistics']::text[];
$$;
-- +goose StatementEnd
COMMENT ON FUNCTION directory.backup_tables() IS 'Schema-nine exhaustive backup dataset allowlist in foreign-key dependency order.';

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_target_issues(p_admin uuid) RETURNS SETOF text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path TO pg_catalog AS $$
DECLARE dataset text; occupied boolean;
BEGIN
 IF (SELECT count(*) FROM identity.users)<>1 OR NOT EXISTS(SELECT 1 FROM identity.users WHERE id=p_admin AND role='SYS_ADMIN' AND access_status='ACTIVE' AND deleted_at IS NULL) THEN
  RETURN NEXT 'target_admin_invalid';
 END IF;
 FOREACH dataset IN ARRAY directory.backup_tables() LOOP
  IF dataset=ANY(ARRAY['identity.users','identity.oauth_identities','identity.email_verification_codes','identity.password_reset_tokens',
   'identity.user_management_permissions','directory.tags','directory.tag_cascades','directory.tag_identity_aliases',
   'directory.software_components','operations.schedules','operations.signing_policy','operations.request_configs',
   'operations.task_settings','operations.platform_backoff']) THEN CONTINUE; END IF;
  EXECUTE format('SELECT EXISTS(SELECT 1 FROM %s)',dataset) INTO occupied;
  IF occupied THEN RETURN NEXT 'target_not_empty'; RETURN; END IF;
 END LOOP;
 IF (SELECT count(*) FROM operations.signing_policy)<>1 OR NOT EXISTS(SELECT 1 FROM operations.signing_policy
  WHERE id=1 AND enabled AND carrier='both' AND ttl_seconds=300 AND revision=1) THEN RETURN NEXT 'target_seed_changed'; END IF;
 IF (SELECT count(*) FROM operations.schedules)<>2 OR EXISTS(SELECT 1 FROM operations.schedules
  WHERE enabled OR archived OR next_at IS NOT NULL OR revision<>1
  OR configuration->>'request_config_id' IS DISTINCT FROM request_config_id::text
  OR configuration-ARRAY['request_config_id'] <> '{"mode":"interval","interval_seconds":3600,"cron":"","timezone":"Asia/Shanghai","scope":"all","site_ids":[],"concurrency":2,"feed_mode":"all","run_timeout_seconds":86400}'::jsonb)
  OR NOT EXISTS(SELECT 1 FROM operations.schedules WHERE kind='monitor' AND name='站点双路检测')
  OR NOT EXISTS(SELECT 1 FROM operations.schedules WHERE kind='rss' AND name='RSS 抓取') THEN RETURN NEXT 'target_seed_changed'; END IF;
 IF (SELECT count(*) FROM operations.request_configs)<>3 OR EXISTS(SELECT 1 FROM operations.request_configs c
  WHERE NOT enabled OR archived OR revision<>1
  OR parameters IS DISTINCT FROM ('{"headers":{},"timeout_seconds":8,"interval_ms":1000,"retries":0,"max_redirects":3,"signed":true,"carrier":"both","ttl_seconds":300}'::jsonb
   || jsonb_build_object('user_agent',CASE WHEN purpose='verification' THEN 'HeyBlog-Verification' ELSE 'HeyBlog/1.0' END))
  OR purpose<>'verification' AND NOT EXISTS(SELECT 1 FROM operations.schedules s WHERE s.request_config_id=c.id AND s.kind=c.purpose AND s.name=c.name)
  OR purpose='verification' AND name<>'HTTP 认证') THEN RETURN NEXT 'target_seed_changed'; END IF;
 IF (SELECT count(*) FROM operations.task_settings)<>1 OR NOT EXISTS(SELECT 1 FROM operations.task_settings s
  JOIN operations.request_configs c ON c.id=s.verification_config_id WHERE s.id=1 AND c.purpose='verification') THEN RETURN NEXT 'target_seed_changed'; END IF;
 IF (SELECT count(*) FROM operations.platform_backoff)<>2 OR EXISTS(SELECT 1 FROM operations.platform_backoff
  WHERE retry_at IS NOT NULL OR reason<>'' OR stage<>0 OR probe_id IS NOT NULL OR probe_until IS NOT NULL) THEN RETURN NEXT 'target_seed_changed'; END IF;
 IF directory.backup_seed_fingerprint()<>'5f865e286d7181374a1d15c0efc64abc' THEN RETURN NEXT 'target_seed_changed'; END IF;
 IF EXISTS(SELECT 1 FROM directory.backup_graph_rows('vertices')) OR EXISTS(SELECT 1 FROM directory.backup_graph_rows('edges')) THEN RETURN NEXT 'target_graph_not_empty'; END IF;
END $$;
-- +goose StatementEnd

-- Extend the deployed protected restore entry points once.
ALTER FUNCTION directory.backup_restore_begin(uuid) RENAME TO backup_restore_begin_v8;
REVOKE ALL ON FUNCTION directory.backup_restore_begin_v8(uuid) FROM PUBLIC, api_runtime;
ALTER FUNCTION directory.backup_restore_finish() RENAME TO backup_restore_finish_v8;
REVOKE ALL ON FUNCTION directory.backup_restore_finish_v8() FROM PUBLIC, api_runtime;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_restore_begin(p_admin uuid) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path TO pg_catalog AS $$
BEGIN
 PERFORM directory.backup_restore_begin_v8(p_admin);
 DELETE FROM operations.task_settings;
 DELETE FROM operations.schedules;
 DELETE FROM operations.request_configs;
 DELETE FROM operations.signing_policy;
 DELETE FROM operations.platform_backoff;
END $$;
-- +goose StatementEnd
COMMENT ON FUNCTION directory.backup_restore_begin(uuid) IS 'Locks schema-nine data and clears initialization seeds within the protected restore scope.';

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_restore_finish() RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path TO pg_catalog AS $$
DECLARE verification_id uuid;
BEGIN
 IF NOT directory.backup_restore_is_active() THEN RAISE EXCEPTION 'restore not active'; END IF;
 UPDATE operations.targets SET state='cancelled',reason='database_restored',finished_at=now()
  WHERE state IN ('pending','running','retry_wait');
 UPDATE operations.targets SET lease_token=NULL,lease_until=NULL,worker_id=NULL;
 UPDATE operations.runs SET state='cancelled',reason='database_restored',finished_at=now(),paused_at=NULL
  WHERE state IN ('queued','running','pausing','paused','cancelling');
 UPDATE operations.workers SET draining=true,heartbeat_at='1970-01-01';
 UPDATE operations.request_attempts SET state='unknown' WHERE state IN ('authorized','sent');
 UPDATE operations.request_statistics s SET unknown=true FROM operations.request_attempts a
  WHERE a.id=s.attempt_id AND a.state='unknown' AND NOT s.received;
 UPDATE operations.platform_backoff SET retry_at=NULL,reason='',stage=0,probe_id=NULL,probe_until=NULL;
 INSERT INTO operations.platform_backoff(platform) VALUES('CF'),('EO') ON CONFLICT DO NOTHING;
 INSERT INTO operations.signing_policy(id) VALUES(1) ON CONFLICT DO NOTHING;
 UPDATE operations.signing_policy SET enabled=true,revision=revision+1;
 UPDATE operations.signing_keys SET state='revoked',revoked_at=COALESCE(revoked_at,now());
 UPDATE operations.request_configs SET enabled=false,parameters=jsonb_set(parameters,'{signed}','true');
 UPDATE operations.schedules SET enabled=false,next_at=NULL;
 IF NOT EXISTS(SELECT 1 FROM operations.request_configs WHERE purpose='verification') THEN
  INSERT INTO operations.request_configs(name,purpose,parameters) VALUES('HTTP 认证','verification',
   '{"user_agent":"HeyBlog-Verification","headers":{},"timeout_seconds":8,"interval_ms":1000,"retries":0,"max_redirects":3,"signed":true,"carrier":"both","ttl_seconds":300}');
 END IF;
 SELECT id INTO verification_id FROM operations.request_configs WHERE purpose='verification' ORDER BY created_at,id LIMIT 1;
 INSERT INTO operations.task_settings VALUES(1,verification_id) ON CONFLICT DO NOTHING;
 IF NOT EXISTS(SELECT 1 FROM operations.schedules) THEN
  WITH configurations AS (
   INSERT INTO operations.request_configs(name,purpose,parameters)
   SELECT name,kind,'{"user_agent":"HeyBlog/1.0","headers":{},"timeout_seconds":8,"interval_ms":1000,"retries":0,"max_redirects":3,"signed":true,"carrier":"both","ttl_seconds":300}'::jsonb
   FROM (VALUES('站点双路检测','monitor'),('RSS 抓取','rss')) defaults(name,kind)
   RETURNING id,name,purpose
  ) INSERT INTO operations.schedules(name,kind,request_config_id,configuration)
   SELECT name,purpose,id,jsonb_build_object('request_config_id',id,'mode','interval','interval_seconds',3600,'cron','','timezone','Asia/Shanghai',
    'scope','all','site_ids','[]'::jsonb,'concurrency',2,'feed_mode','all','run_timeout_seconds',86400) FROM configurations;
 END IF;
 DELETE FROM directory.site_monitoring_rounds;
 DELETE FROM operations.route_state;
 UPDATE directory.sites SET status=NULL,access_scope=NULL,last_checked_at=NULL;
 PERFORM directory.backup_restore_finish_v8();
END $$;
-- +goose StatementEnd
COMMENT ON FUNCTION directory.backup_restore_finish() IS 'Invalidates restored execution and signing capabilities while preserving history, business content and cumulative statistics.';

-- +goose StatementBegin
DO $migration$
DECLARE expected text;
BEGIN
 SELECT md5(string_agg(n.nspname||'.'||c.relname||':'||a.attname||':'||a.atttypid::regtype::text||':'||a.attgenerated::text,',' ORDER BY n.nspname,c.relname,a.attnum)) INTO expected
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
 WHERE n.nspname IN ('identity','directory','content','operations') AND c.relkind IN ('r','p');
 EXECUTE format($definition$
 CREATE OR REPLACE FUNCTION directory.backup_check_schema() RETURNS void
 LANGUAGE plpgsql STABLE SET search_path TO pg_catalog AS $body$
 DECLARE signature text;
 BEGIN
 SELECT md5(string_agg(n.nspname||'.'||c.relname||':'||a.attname||':'||a.atttypid::regtype::text||':'||a.attgenerated::text,',' ORDER BY n.nspname,c.relname,a.attnum)) INTO signature
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
 WHERE n.nspname IN ('identity','directory','content','operations') AND c.relkind IN ('r','p');
 IF signature<>%L THEN RAISE EXCEPTION 'unsupported backup schema' USING ERRCODE='22023'; END IF;
 END $body$;
 $definition$,expected);
END $migration$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION directory.backup_restore_begin(uuid), directory.backup_restore_finish() FROM PUBLIC;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_restore_finish(p_schema_version integer) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog'
    AS $$
BEGIN
    IF p_schema_version IS NULL OR p_schema_version NOT BETWEEN 3 AND 9 THEN
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

GRANT EXECUTE ON FUNCTION directory.backup_restore_begin(uuid), directory.backup_restore_finish(integer) TO api_runtime;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Forward-only schema-nine migration; restore the previous database backup.'; END $$;
-- +goose StatementEnd

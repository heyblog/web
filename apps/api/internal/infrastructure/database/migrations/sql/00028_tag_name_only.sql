-- +goose Up
SELECT pg_advisory_xact_lock(74821953);
SET CONSTRAINTS ALL IMMEDIATE;
DROP VIEW directory.tag_dictionary;
DROP FUNCTION directory.write_tag_dictionary();
DROP FUNCTION directory.fill_selected_tag_labels() CASCADE;
DROP FUNCTION directory.check_tag_label_ownership() CASCADE;
DROP FUNCTION directory.canonical_tag_slugs(text[]);
DROP FUNCTION directory.canonical_tag_slug(text);
DROP TABLE directory.tag_slug_aliases;
DROP TABLE directory.slug_generation_cache;
DROP TABLE directory.slug_generation_jobs;
DROP TABLE content.system_ai_settings;

-- Snapshot every old label before dropping its representation. Each role joins
-- this map independently, including paths with the same concept in both roles.
CREATE TEMP TABLE tag_name_map ON COMMIT DROP AS
SELECT l.id AS label_id,l.tag_id AS old_tag_id,
 CASE WHEN l.id=t.default_label_id THEN t.id ELSE uuidv7() END AS tag_id,
 l.name,t.description,t.is_enabled AND l.is_enabled AS is_enabled,
 l.created_at,l.updated_at,to_jsonb(l) AS snapshot,
 l.id=t.default_label_id AS is_default
FROM directory.tag_labels l JOIN directory.tags t ON t.id=l.tag_id;
ALTER TABLE directory.tags DISABLE TRIGGER tags_touch_updated_at;
ALTER TABLE directory.tags DROP COLUMN default_label_id, DROP COLUMN slug;
ALTER TABLE directory.tags
    ADD COLUMN name text CHECK (char_length(btrim(name)) BETWEEN 1 AND 120); -- Human-readable tag name.
UPDATE directory.tags t SET name=m.name FROM tag_name_map m WHERE m.tag_id=t.id AND m.is_default;
ALTER TABLE directory.tags ALTER COLUMN name SET NOT NULL;
ALTER TABLE directory.tags
    ADD COLUMN normalized_name text NOT NULL GENERATED ALWAYS AS (lower(btrim(name))) STORED UNIQUE; -- Unique lowercase trimmed tag name generated from name.
INSERT INTO directory.tags(id,name,description,is_enabled,created_at,updated_at)
SELECT tag_id,name,description,is_enabled,created_at,updated_at FROM tag_name_map WHERE NOT is_default;
ALTER TABLE directory.tags ENABLE TRIGGER tags_touch_updated_at;

ALTER TABLE directory.tag_identity_aliases DROP CONSTRAINT tag_identity_aliases_pkey;
ALTER TABLE directory.tag_identity_aliases
    ADD COLUMN alias_kind text NOT NULL DEFAULT 'TAG' CHECK (alias_kind IN ('TAG','LABEL')); -- Historical identity namespace: TAG or LABEL; only TAG may have a system key.
ALTER TABLE directory.tag_identity_aliases ADD PRIMARY KEY(alias_kind,alias_id);
ALTER TABLE directory.tag_identity_aliases ADD CONSTRAINT label_alias_no_system_key CHECK(alias_kind='TAG' OR system_key IS NULL);
INSERT INTO directory.tag_identity_aliases(alias_kind,alias_id,tag_id,snapshot)
SELECT 'LABEL',label_id,tag_id,snapshot FROM tag_name_map;

ALTER TABLE directory.tag_cascades
    ALTER COLUMN sort_order TYPE integer; -- Unique positive order within scope; expanded paths append after existing paths.
CREATE TEMP TABLE tag_path_map ON COMMIT DROP AS
SELECT c.id AS old_id,p.label_id AS primary_label_id,s.label_id AS secondary_label_id,
 CASE WHEN p.is_default AND s.is_default THEN c.id ELSE uuidv7() END AS id,
 p.tag_id AS level1_tag_id,s.tag_id AS level2_tag_id,
 p.is_default AND s.is_default AS is_default,
 p.is_enabled AND s.is_enabled AS labels_enabled
FROM directory.tag_cascades c
JOIN tag_name_map p ON p.old_tag_id=c.level1_tag_id
JOIN tag_name_map s ON s.old_tag_id=c.level2_tag_id;
INSERT INTO directory.tag_cascades(id,scope,taxonomy_key,level1_tag_id,level2_tag_id,sort_order,is_enabled,created_at,updated_at,merged_into_id)
SELECT m.id,c.scope,'managed/'||m.id,m.level1_tag_id,m.level2_tag_id,
 (SELECT max(old.sort_order) FROM directory.tag_cascades old WHERE old.scope=c.scope)
 +row_number() OVER(PARTITION BY c.scope ORDER BY c.sort_order,m.primary_label_id,m.secondary_label_id)::integer,
 c.is_enabled AND m.labels_enabled AND c.merged_into_id IS NULL,c.created_at,c.updated_at,c.merged_into_id
FROM tag_path_map m JOIN directory.tag_cascades c ON c.id=m.old_id WHERE NOT m.is_default;

ALTER TABLE directory.sites DISABLE TRIGGER sites_ensure_cascade_scope;
ALTER TABLE directory.sites DISABLE TRIGGER sites_touch_site;
ALTER TABLE content.articles DISABLE TRIGGER articles_ensure_cascade_scope;
ALTER TABLE content.articles DISABLE TRIGGER articles_touch_updated_at;
ALTER TABLE directory.site_tags DISABLE TRIGGER site_tags_ensure_assignment;
ALTER TABLE content.article_tags DISABLE TRIGGER article_tags_ensure_assignment;
UPDATE directory.sites s SET tag_cascade_id=m.id FROM tag_path_map m
WHERE m.old_id=s.tag_cascade_id AND m.primary_label_id=s.primary_label_id AND m.secondary_label_id=s.secondary_label_id;
UPDATE content.articles a SET tag_cascade_id=m.id FROM tag_path_map m
WHERE m.old_id=a.tag_cascade_id AND m.primary_label_id=a.primary_label_id AND m.secondary_label_id=a.secondary_label_id;
UPDATE directory.site_tags a SET tag_id=m.tag_id FROM tag_name_map m WHERE m.label_id=a.label_id;
UPDATE content.article_tags a SET tag_id=m.tag_id FROM tag_name_map m WHERE m.label_id=a.label_id;
SET CONSTRAINTS ALL IMMEDIATE;

-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM directory.sites s JOIN directory.tag_cascades c ON c.id=s.tag_cascade_id
   WHERE c.scope<>'SITE') OR EXISTS(SELECT 1 FROM content.articles a JOIN directory.tag_cascades c ON c.id=a.tag_cascade_id WHERE c.scope<>'ARTICLE')
 OR EXISTS(SELECT 1 FROM directory.site_tags a JOIN directory.sites s ON s.id=a.site_id JOIN directory.tag_cascades c ON c.id=s.tag_cascade_id WHERE a.role='TERTIARY' AND a.tag_id IN(c.level1_tag_id,c.level2_tag_id))
 OR EXISTS(SELECT 1 FROM content.article_tags a JOIN content.articles s ON s.id=a.article_id JOIN directory.tag_cascades c ON c.id=s.tag_cascade_id WHERE a.role='TERTIARY' AND a.tag_id IN(c.level1_tag_id,c.level2_tag_id))
 THEN RAISE EXCEPTION 'name-only migration violates classification invariants'; END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE directory.sites ENABLE TRIGGER sites_ensure_cascade_scope;
ALTER TABLE directory.sites ENABLE TRIGGER sites_touch_site;
ALTER TABLE content.articles ENABLE TRIGGER articles_ensure_cascade_scope;
ALTER TABLE content.articles ENABLE TRIGGER articles_touch_updated_at;
ALTER TABLE directory.site_tags ENABLE TRIGGER site_tags_ensure_assignment;
ALTER TABLE content.article_tags ENABLE TRIGGER article_tags_ensure_assignment;
ALTER TABLE directory.sites DROP COLUMN primary_label_id, DROP COLUMN secondary_label_id;
ALTER TABLE content.articles DROP COLUMN primary_label_id, DROP COLUMN secondary_label_id;
ALTER TABLE directory.site_tags DROP COLUMN label_id;
ALTER TABLE content.article_tags DROP COLUMN label_id;
DROP TABLE directory.tag_labels;
COMMENT ON TABLE directory.tags IS 'Unique names; classification roles belong to paths and assignments.';
COMMENT ON COLUMN directory.tags.name IS 'Human-readable tag name.';
COMMENT ON COLUMN directory.tags.normalized_name IS 'Unique lowercase trimmed tag name generated from name.';
COMMENT ON COLUMN directory.tag_identity_aliases.alias_kind IS 'Historical identity namespace: TAG or LABEL; only TAG may have a system key.';
COMMENT ON COLUMN directory.tag_cascades.sort_order IS 'Unique positive order within scope; expanded paths append after existing paths.';

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_tables() RETURNS text[] LANGUAGE sql IMMUTABLE AS $$
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
CREATE OR REPLACE FUNCTION directory.backup_seed_fingerprint() RETURNS text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
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
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_target_issues(p_admin uuid) RETURNS SETOF text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
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
 IF directory.backup_seed_fingerprint()<>'56791cf980237c5a51019028c30c6780' THEN RETURN NEXT 'target_seed_changed'; END IF;
 IF EXISTS(SELECT 1 FROM directory.backup_graph_rows('vertices')) OR EXISTS(SELECT 1 FROM directory.backup_graph_rows('edges')) THEN RETURN NEXT 'target_graph_not_empty'; END IF;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_restore_begin(p_admin uuid) RETURNS void
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
 DELETE FROM directory.tag_identity_aliases;
 DELETE FROM directory.tag_cascades;
 DELETE FROM directory.tags;
 DELETE FROM directory.software_components;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.backup_check_schema() RETURNS void LANGUAGE plpgsql STABLE SET search_path=pg_catalog AS $$
DECLARE signature text;
BEGIN
 SELECT md5(string_agg(n.nspname||'.'||c.relname||':'||a.attname||':'||a.atttypid::regtype::text||':'||a.attgenerated::text,',' ORDER BY n.nspname,c.relname,a.attnum)) INTO signature
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
 WHERE n.nspname IN ('identity','directory','content') AND c.relkind IN ('r','p');
 IF signature<>'591864c8c3d0a0a650ed94cceb3c42f5' THEN RAISE EXCEPTION 'unsupported backup schema' USING ERRCODE='22023'; END IF;
END $$;
-- +goose StatementEnd

COMMENT ON FUNCTION directory.backup_tables() IS 'Protocol-one schema-three exhaustive application backup dataset allowlist in dependency order.';

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Migration 00028 is forward-only; restore the pre-upgrade database backup.'; END $$;
-- +goose StatementEnd

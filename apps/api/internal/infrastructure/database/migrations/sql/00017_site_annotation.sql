-- +goose Up

ALTER TABLE directory.sites
    ADD COLUMN blog_status text NOT NULL DEFAULT 'UNREVIEWED', -- Whether the site is an eligible blog, a confirmed non-blog, or awaiting annotation.
    ADD CONSTRAINT sites_blog_status_check CHECK (blog_status IN ('UNREVIEWED', 'BLOG', 'NON_BLOG'));

COMMENT ON COLUMN directory.sites.blog_status IS 'Whether the site is an eligible blog, a confirmed non-blog, or awaiting annotation.';

ALTER TABLE directory.sites DISABLE TRIGGER sites_touch_site;
UPDATE directory.sites AS site
   SET blog_status = 'BLOG'
 WHERE EXISTS (
       SELECT 1 FROM directory.site_audits AS audit
        WHERE audit.site_id = site.id AND audit.status = 'APPROVED'
          AND audit.action IN ('CREATE', 'UPDATE')
 );
ALTER TABLE directory.sites ENABLE TRIGGER sites_touch_site;

ALTER TABLE identity.user_management_permissions DROP CONSTRAINT user_management_permissions_key_check;
ALTER TABLE identity.user_management_permissions
    ADD CONSTRAINT user_management_permissions_key_check CHECK (permission_key IN (
        'user.manage', 'site_audit.review', 'feedback.review', 'announcement.manage',
        'taxonomy.manage', 'site.manage', 'task.manage', 'log.read', 'site_annotation.manage'
    ));

CREATE TABLE directory.site_annotation_history (
    id uuid NOT NULL DEFAULT uuidv7() PRIMARY KEY, -- Change record identifier.
    site_id uuid NOT NULL REFERENCES directory.sites(id) ON DELETE RESTRICT, -- Site whose data was changed.
    actor_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT, -- Administrator who applied the change.
    site_revision bigint NOT NULL CHECK (site_revision >= 1), -- Site revision after the change.
    source text NOT NULL CHECK (source IN ('ANNOTATION', 'AUDIT')), -- Whether the change came from the annotation desk or submission review.
    site_audit_id uuid REFERENCES directory.site_audits(id) ON DELETE RESTRICT, -- Submission review that changed the blog status, if any.
    reason text, -- Optional change explanation.
    before_snapshot jsonb NOT NULL CHECK (jsonb_typeof(before_snapshot) = 'object'), -- Previous values of the changed data.
    after_snapshot jsonb NOT NULL CHECK (jsonb_typeof(after_snapshot) = 'object'), -- New values of the changed data.
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(), -- Time the change was recorded.
    CONSTRAINT site_annotation_history_origin_check CHECK ((source = 'AUDIT') = (site_audit_id IS NOT NULL)),
    CONSTRAINT site_annotation_history_revision_unique UNIQUE (site_id, site_revision)
);

COMMENT ON TABLE directory.site_annotation_history IS 'Append-only record of manual annotations and blog classification changes from submission review.';
COMMENT ON COLUMN directory.site_annotation_history.id IS 'Change record identifier.';
COMMENT ON COLUMN directory.site_annotation_history.site_id IS 'Site whose data was changed.';
COMMENT ON COLUMN directory.site_annotation_history.actor_id IS 'Administrator who applied the change.';
COMMENT ON COLUMN directory.site_annotation_history.site_revision IS 'Site revision after the change.';
COMMENT ON COLUMN directory.site_annotation_history.source IS 'Whether the change came from the annotation desk or submission review.';
COMMENT ON COLUMN directory.site_annotation_history.site_audit_id IS 'Submission review that changed the blog status, if any.';
COMMENT ON COLUMN directory.site_annotation_history.reason IS 'Optional change explanation.';
COMMENT ON COLUMN directory.site_annotation_history.before_snapshot IS 'Previous values of the changed data.';
COMMENT ON COLUMN directory.site_annotation_history.after_snapshot IS 'New values of the changed data.';
COMMENT ON COLUMN directory.site_annotation_history.created_at IS 'Time the change was recorded.';

CREATE INDEX sites_annotation_list_idx ON directory.sites (blog_status, updated_at DESC, id DESC);
GRANT SELECT, INSERT ON directory.site_annotation_history TO api_runtime;

DROP TRIGGER invalidate_public_graph ON directory.sites;
CREATE TRIGGER invalidate_public_graph AFTER INSERT OR DELETE OR TRUNCATE
    OR UPDATE OF name, short_id, custom_id, scheme, normalized_host, base_path, visibility, blog_status ON directory.sites
    FOR EACH STATEMENT EXECUTE FUNCTION runtime_cache.invalidate_public_graph();

COMMENT ON TRIGGER invalidate_public_graph ON directory.sites IS 'Invalidates the public graph after changes to site identity or eligibility.';

-- +goose StatementBegin
CREATE FUNCTION directory.list_recorded_friend_links(p_source_site_id uuid) RETURNS TABLE(target_site_id uuid, target_host text, target_url text, link_status text, is_reciprocal boolean, created_at_ms bigint, updated_at_ms bigint)
    LANGUAGE plpgsql STABLE SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'ag_catalog', 'directory'
    AS $_$
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
         ORDER BY link.resolved_host, link.resolved_url
        $query$,
        to_json(p_source_site_id::text)::text
    );
END;
$_$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION directory.list_recorded_incoming_links(p_target_host text)
RETURNS TABLE(source_site_id uuid, link_status text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path TO 'pg_catalog'
AS $$
    SELECT link.source_site_id, link.link_status FROM directory.graph_incoming_links(p_target_host) AS link;
$$;
-- +goose StatementEnd

COMMENT ON FUNCTION directory.list_recorded_friend_links(uuid) IS 'Reads outgoing graph edges for site annotation, including inactive and unregistered targets.';
COMMENT ON FUNCTION directory.list_recorded_incoming_links(text) IS 'Reads incoming graph edges for site annotation, including inactive links.';

REVOKE ALL ON FUNCTION directory.list_recorded_friend_links(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION directory.list_recorded_incoming_links(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION directory.list_recorded_friend_links(uuid) TO api_runtime;
GRANT EXECUTE ON FUNCTION directory.list_recorded_incoming_links(text) TO api_runtime;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.list_friend_links(p_source_site_id uuid, p_include_inactive boolean DEFAULT false)
RETURNS TABLE(target_site_id uuid, target_host text, target_url text, link_status text, is_reciprocal boolean, created_at_ms bigint, updated_at_ms bigint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path TO 'pg_catalog'
AS $$
    SELECT link.*
      FROM directory.sites AS source
      CROSS JOIN LATERAL directory.list_recorded_friend_links(source.id) AS link
      LEFT JOIN directory.sites AS target ON target.id = link.target_site_id
     WHERE source.id = p_source_site_id AND source.blog_status = 'BLOG' AND source.visibility = 'VISIBLE'
       AND (p_include_inactive OR link.link_status = 'ACTIVE')
       AND target.blog_status = 'BLOG' AND target.visibility = 'VISIBLE'
     ORDER BY link.target_host, link.target_url;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.list_owner_friend_links(p_source_site_id uuid, p_include_inactive boolean DEFAULT false)
RETURNS TABLE(target_site_id uuid, target_host text, target_url text, link_status text, is_reciprocal boolean, created_at_ms bigint, updated_at_ms bigint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path TO 'pg_catalog'
AS $$
    SELECT link.*
      FROM directory.sites AS source
      CROSS JOIN LATERAL directory.list_recorded_friend_links(source.id) AS link
      LEFT JOIN directory.sites AS target ON target.id = link.target_site_id
     WHERE source.id = p_source_site_id AND source.blog_status = 'BLOG' AND source.visibility <> 'REMOVED'
       AND (p_include_inactive OR link.link_status = 'ACTIVE')
       AND (link.target_site_id IS NULL OR target.blog_status = 'BLOG')
     ORDER BY link.target_host, link.target_url;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.public_friend_graph(p_center_id uuid DEFAULT NULL::uuid) RETURNS jsonb
    LANGUAGE plpgsql STABLE SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'ag_catalog', 'directory'
    AS $_$
DECLARE
    center_short_id text;
    center_visibility text;
    result jsonb;
BEGIN
    IF p_center_id IS NOT NULL THEN
        SELECT short_id, visibility INTO center_short_id, center_visibility
          FROM directory.sites WHERE id = p_center_id AND blog_status = 'BLOG';
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
                   'site:' || target.short_id AS target
              FROM raw_edges AS edge
              JOIN directory.sites AS source ON source.id = edge.source_id AND source.visibility = 'VISIBLE' AND source.blog_status = 'BLOG'
              JOIN directory.sites AS target ON target.id = edge.target_id AND target.visibility = 'VISIBLE' AND target.blog_status = 'BLOG'
        ), nodes AS (
            SELECT 'site:' || site.short_id AS id, site.name, site.normalized_host AS host,
                   site.scheme || '://' || site.normalized_host || site.base_path AS homepage_url,
                   site.short_id, site.custom_id
              FROM directory.sites AS site
             WHERE site.blog_status = 'BLOG' AND site.visibility = 'VISIBLE' AND (%L::uuid IS NULL OR site.id = %L::uuid
                OR EXISTS (SELECT 1 FROM visible_edges AS edge
                            WHERE edge.source = 'site:' || site.short_id OR edge.target = 'site:' || site.short_id))
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
$_$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.apply_owner_audit_effects() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    request_row directory.owner_friend_link_requests;
    owner_row directory.site_ownerships;
    target_row directory.sites;
    final_url text;
    v_source_key text;
    v_source_id uuid;
BEGIN
    IF OLD.source_channel IS DISTINCT FROM NEW.source_channel
       OR OLD.source_site_id IS DISTINCT FROM NEW.source_site_id
       OR OLD.ownership_id IS DISTINCT FROM NEW.ownership_id
       OR (OLD.submitter_user_id IS DISTINCT FROM NEW.submitter_user_id AND (NEW.submitter_user_id IS NOT NULL OR EXISTS (SELECT 1 FROM identity.users WHERE id = OLD.submitter_user_id AND deleted_at IS NULL))) THEN
        RAISE EXCEPTION 'audit provenance is immutable';
    END IF;
    IF OLD.status <> 'PENDING' OR NEW.status = 'PENDING' THEN RETURN NEW; END IF;
    IF NEW.status = 'REJECTED' THEN
        UPDATE directory.owner_friend_link_requests SET status = 'REJECTED' WHERE audit_id = NEW.id AND status = 'PENDING';
        RETURN NEW;
    END IF;
    -- Let the existing approval constraint reject a missing target before applying effects.
    IF NEW.site_id IS NULL THEN RETURN NEW; END IF;
    SELECT * INTO STRICT target_row FROM directory.sites WHERE id = NEW.site_id;
    final_url := target_row.scheme || '://' || target_row.normalized_host || target_row.base_path;
    IF NEW.source_channel = 'OWNER_UPDATE' THEN
        PERFORM directory.apply_verified_site_address(NEW.site_id, NEW.submitter_user_id, final_url, NEW.ownership_id);
    END IF;
    IF NEW.action = 'CREATE' AND NEW.source_channel <> 'ANONYMOUS' THEN
        v_source_key := CASE WHEN NEW.source_channel = 'OWNER_FRIEND_LINK' THEN 'OWNER_FRIEND_LINK' ELSE 'ACCOUNT_SUBMISSION' END;
        INSERT INTO directory.site_sources (source_key, name) VALUES (v_source_key, CASE WHEN v_source_key = 'OWNER_FRIEND_LINK' THEN '站主友链推荐' ELSE '用户提交' END)
        ON CONFLICT (source_key) DO UPDATE SET is_enabled = true RETURNING id INTO v_source_id;
        INSERT INTO directory.site_origins (site_id, source_id, external_reference, metadata)
        VALUES (NEW.site_id, v_source_id, NEW.id::text, jsonb_build_object('channel', NEW.source_channel, 'user_id', NEW.submitter_user_id, 'source_site_id', NEW.source_site_id, 'audit_id', NEW.id))
        ON CONFLICT (site_id, source_id) DO NOTHING;
    END IF;
    FOR request_row IN SELECT * FROM directory.owner_friend_link_requests WHERE audit_id = NEW.id AND status = 'PENDING' ORDER BY source_site_id FOR UPDATE LOOP
        PERFORM 1 FROM directory.sites WHERE id = request_row.source_site_id FOR UPDATE;
        SELECT o.* INTO owner_row FROM directory.site_ownerships o JOIN directory.sites s ON s.id = o.site_id
         WHERE o.id = request_row.ownership_id AND o.user_id = request_row.user_id
           AND s.blog_status = 'BLOG' AND s.visibility <> 'REMOVED' AND o.address = s.scheme || '://' || s.normalized_host || s.base_path FOR UPDATE OF o;
        IF owner_row.id IS NULL OR owner_row.site_id = NEW.site_id THEN
            UPDATE directory.owner_friend_link_requests SET status = 'CANCELLED' WHERE id = request_row.id;
        ELSE
            PERFORM directory.upsert_registered_friend_link(request_row.source_site_id, final_url, target_row.normalized_host, 'ACTIVE');
            INSERT INTO directory.site_sources (source_key, name) VALUES ('OWNER_FRIEND_LINK', '站主友链推荐')
            ON CONFLICT (source_key) DO UPDATE SET is_enabled = true RETURNING id INTO v_source_id;
            INSERT INTO directory.site_origins (site_id, source_id, external_reference, metadata)
            VALUES (NEW.site_id, v_source_id, NEW.id::text, jsonb_build_object('channel', 'OWNER_FRIEND_LINK', 'recommendations', jsonb_build_array(jsonb_build_object('user_id', request_row.user_id, 'source_site_id', request_row.source_site_id, 'audit_id', NEW.id))))
            ON CONFLICT (site_id, source_id) DO UPDATE SET metadata = directory.site_origins.metadata || jsonb_build_object('recommendations', COALESCE(directory.site_origins.metadata -> 'recommendations', '[]'::jsonb) || (EXCLUDED.metadata -> 'recommendations'));
            UPDATE directory.owner_friend_link_requests SET status = 'APPLIED' WHERE id = request_row.id;
        END IF;
    END LOOP;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.apply_verified_site_address(p_site uuid, p_user uuid, p_address text, p_ownership uuid) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE v_owner directory.site_ownerships%ROWTYPE; v_claim uuid;
BEGIN
 PERFORM 1 FROM directory.sites WHERE id=p_site AND blog_status = 'BLOG' AND visibility <> 'REMOVED' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'site unavailable' USING ERRCODE='23514',CONSTRAINT='site_ownership_changed'; END IF;
 SELECT * INTO v_owner FROM directory.site_ownerships WHERE site_id=p_site AND id=p_ownership AND user_id=p_user FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'site ownership changed' USING ERRCODE='23514',CONSTRAINT='site_ownership_changed'; END IF;
 IF v_owner.address=p_address THEN RETURN; END IF;
 SELECT id INTO v_claim FROM directory.site_claims WHERE site_id=p_site AND user_id=p_user AND address=p_address
 AND status='VERIFIED' AND consumed_at IS NULL AND created_at>=v_owner.created_at ORDER BY verified_at DESC LIMIT 1 FOR UPDATE;
 IF v_claim IS NULL THEN RAISE EXCEPTION 'matching verified address required' USING ERRCODE='23514',CONSTRAINT='verified_site_address_required'; END IF;
 UPDATE directory.site_claims SET consumed_at=now() WHERE id=v_claim;
 UPDATE directory.site_ownerships SET address=p_address,verified_claim_id=v_claim,revision=revision+1,updated_at=now() WHERE id=v_owner.id;
 INSERT INTO directory.site_ownership_events(site_id,user_id,actor_id,action,reason) VALUES(p_site,p_user,p_user,'ADDRESS_CHANGED','Approved site address update');
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.validate_owner_audit_submission() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    owner_row directory.site_ownerships;
    current_url text;
    proposed_url text;
BEGIN
    IF directory.backup_restore_is_active() THEN RETURN NEW; END IF;
    IF NEW.source_channel NOT IN ('OWNER_UPDATE', 'OWNER_FRIEND_LINK') THEN RETURN NEW; END IF;
    SELECT scheme || '://' || normalized_host || base_path INTO current_url
      FROM directory.sites WHERE id = NEW.source_site_id AND blog_status = 'BLOG' AND visibility <> 'REMOVED' FOR UPDATE;
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

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.move_site_ref(p_site_id uuid, p_old_host text, p_new_host text, p_new_scheme text, p_new_base_path text) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'ag_catalog', 'directory'
    AS $_$
DECLARE
    incoming record;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM directory.sites
         WHERE id = p_site_id AND normalized_host = p_new_host
           AND scheme = p_new_scheme AND base_path = p_new_base_path
    ) THEN
        RAISE EXCEPTION 'site move does not match the current directory site';
    END IF;

    PERFORM pg_advisory_xact_lock(hashtextextended('site-ref:' || LEAST(p_old_host, p_new_host), 0));
    PERFORM pg_advisory_xact_lock(hashtextextended('site-ref:' || GREATEST(p_old_host, p_new_host), 0));

    IF p_old_host <> p_new_host THEN
        -- The previous outgoing edge to our future address would become a self-link.
        FOR incoming IN SELECT * FROM directory.graph_incoming_links(p_new_host) WHERE source_site_id <> p_site_id
        LOOP
            PERFORM pg_advisory_xact_lock(hashtextextended(
                'friend-link:' || incoming.source_site_id::text || ':' || p_new_host, 0
            ));
            PERFORM directory.merge_friend_link_graph(
                incoming.source_site_id,
                p_old_host,
                directory.canonical_site_url(p_new_scheme, p_old_host, p_new_base_path),
                incoming.link_status,
                incoming.created_at_ms,
                incoming.updated_at_ms
            );
        END LOOP;

        EXECUTE format(
            $query$
            SELECT * FROM ag_catalog.cypher('directory_graph', $cypher$
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
            SELECT * FROM ag_catalog.cypher('directory_graph', $cypher$
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
            directory.canonical_site_url(p_new_scheme, p_new_host, p_new_base_path),
            incoming.link_status,
            incoming.created_at_ms,
            incoming.updated_at_ms
        );
    END LOOP;
END;
$_$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Cannot downgrade site classification; restore the previous database backup.'; END $$;
-- +goose StatementEnd

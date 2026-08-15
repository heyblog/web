-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION directory.upsert_registered_friend_link(
    p_source_site_id uuid,
    p_target_url text,
    p_target_host text,
    p_status text DEFAULT 'ACTIVE'
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, ag_catalog, directory
AS $function$
DECLARE
    source_host text;
    event_ms bigint;
BEGIN
    IF p_status NOT IN ('ACTIVE', 'INACTIVE') THEN
        RAISE EXCEPTION 'friend-link status must be ACTIVE or INACTIVE';
    END IF;
    IF p_target_host <> lower(btrim(p_target_host)) OR p_target_host = '' THEN
        RAISE EXCEPTION 'friend-link target host must be normalized';
    END IF;
    IF substring(p_target_url FROM '^https?://([^/?#]+)') IS DISTINCT FROM p_target_host
       OR p_target_url ~ '#' THEN
        RAISE EXCEPTION 'friend-link URL must be an absolute normalized URL without a fragment';
    END IF;

    SELECT normalized_host INTO source_host FROM directory.sites WHERE id = p_source_site_id;
    IF source_host IS NULL THEN
        RAISE EXCEPTION 'friend-link source site does not exist';
    END IF;
    IF source_host = p_target_host THEN
        RAISE EXCEPTION 'friend-link self edges are not allowed';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM directory.sites WHERE normalized_host = p_target_host) THEN
        RAISE EXCEPTION 'registered friend-link target site does not exist';
    END IF;

    PERFORM pg_advisory_xact_lock(hashtextextended('friend-link:' || p_source_site_id::text || ':' || p_target_host, 0));
    event_ms = (extract(epoch FROM clock_timestamp()) * 1000)::bigint;
    PERFORM directory.merge_friend_link_graph(
        p_source_site_id,
        p_target_host,
        p_target_url,
        p_status,
        event_ms,
        event_ms
    );
END;
$function$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION directory.upsert_registered_friend_link(uuid, text, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION directory.upsert_registered_friend_link(uuid, text, text, text) TO api_runtime;

COMMENT ON FUNCTION directory.upsert_registered_friend_link(uuid, text, text, text) IS
'Creates or updates one directed FRIEND_LINK edge when both endpoint sites are already registered.';

-- +goose Down
REVOKE EXECUTE ON FUNCTION directory.upsert_registered_friend_link(uuid, text, text, text) FROM api_runtime;
DROP FUNCTION directory.upsert_registered_friend_link(uuid, text, text, text);

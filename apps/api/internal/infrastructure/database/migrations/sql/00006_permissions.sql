-- +goose Up
REVOKE ALL ON SCHEMA identity FROM PUBLIC;
REVOKE ALL ON SCHEMA directory FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA identity FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA directory FROM PUBLIC;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA identity FROM PUBLIC;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA directory FROM PUBLIC;

GRANT USAGE ON SCHEMA identity, directory TO api_runtime;
GRANT SELECT, INSERT, UPDATE ON identity.users TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON identity.oauth_identities TO api_runtime;
GRANT SELECT, INSERT, UPDATE ON directory.sites TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON directory.site_feeds TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON directory.site_resources TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON directory.site_icons TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON directory.tags TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON directory.site_tags TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON directory.software_components TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON directory.software_component_dependencies TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON directory.site_software_components TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON directory.site_sources TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON directory.site_origins TO api_runtime;
GRANT EXECUTE ON FUNCTION directory.upsert_friend_link(uuid, text, text, text) TO api_runtime;
GRANT EXECUTE ON FUNCTION directory.list_friend_links(uuid, boolean) TO api_runtime;

-- +goose Down
REVOKE EXECUTE ON FUNCTION directory.list_friend_links(uuid, boolean) FROM api_runtime;
REVOKE EXECUTE ON FUNCTION directory.upsert_friend_link(uuid, text, text, text) FROM api_runtime;
REVOKE ALL ON ALL TABLES IN SCHEMA directory FROM api_runtime;
REVOKE ALL ON ALL TABLES IN SCHEMA identity FROM api_runtime;
REVOKE USAGE ON SCHEMA directory, identity FROM api_runtime;

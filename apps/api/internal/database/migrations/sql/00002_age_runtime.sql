-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    required_role text;
BEGIN
    FOREACH required_role IN ARRAY ARRAY['migrator', 'api_runtime']
    LOOP
        IF NOT EXISTS (
            SELECT 1
              FROM pg_roles AS role
              JOIN pg_db_role_setting AS setting ON setting.setrole = role.oid
             WHERE role.rolname = required_role
               AND setting.setdatabase = 0
               AND 'session_preload_libraries=age' = ANY(setting.setconfig)
        ) THEN
            RAISE EXCEPTION '% role is missing the AGE session preload setting', required_role;
        END IF;
    END LOOP;
END;
$$;
-- +goose StatementEnd

-- +goose Down
SELECT 1;

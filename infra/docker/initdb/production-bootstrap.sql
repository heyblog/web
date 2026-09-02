\set ON_ERROR_STOP on

\getenv migrator_password POSTGRES_MIGRATOR_PASSWORD
\getenv runtime_password POSTGRES_RUNTIME_PASSWORD

\if :{?migrator_password}
\else
\echo 'POSTGRES_MIGRATOR_PASSWORD is required' >&2
\quit 3
\endif

\if :{?runtime_password}
\else
\echo 'POSTGRES_RUNTIME_PASSWORD is required' >&2
\quit 3
\endif

SELECT length(:'migrator_password') > 0 AS migrator_password_valid,
       length(:'runtime_password') > 0 AS runtime_password_valid
\gset

\if :migrator_password_valid
\else
\echo 'POSTGRES_MIGRATOR_PASSWORD must not be empty' >&2
\quit 3
\endif

\if :runtime_password_valid
\else
\echo 'POSTGRES_RUNTIME_PASSWORD must not be empty' >&2
\quit 3
\endif

DO $$
BEGIN
    IF current_setting('server_version_num')::integer / 10000 <> 18 THEN
        RAISE EXCEPTION 'HeyBlog requires PostgreSQL 18';
    END IF;

    IF current_setting('max_locks_per_transaction')::integer < 512 THEN
        RAISE EXCEPTION 'max_locks_per_transaction must be at least 512';
    END IF;

    IF NOT EXISTS (
        SELECT 1
          FROM pg_available_extensions
         WHERE name = 'age'
    ) THEN
        RAISE EXCEPTION 'Apache AGE is not installed for this PostgreSQL server';
    END IF;
END;
$$;

SELECT format(
    'CREATE ROLE migrator LOGIN PASSWORD %L NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS',
    :'migrator_password'
)
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'migrator')
\gexec

ALTER ROLE migrator
    LOGIN
    PASSWORD :'migrator_password'
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOREPLICATION
    NOBYPASSRLS;
COMMENT ON ROLE migrator IS 'Owns migration metadata and application database objects without cluster administration privileges.';

SELECT format(
    'CREATE ROLE api_runtime LOGIN PASSWORD %L NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS',
    :'runtime_password'
)
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'api_runtime')
\gexec

ALTER ROLE api_runtime
    LOGIN
    PASSWORD :'runtime_password'
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOREPLICATION
    NOBYPASSRLS;
COMMENT ON ROLE api_runtime IS 'Runs the API with explicit business DML and typed graph-function permissions only.';

ALTER ROLE migrator SET session_preload_libraries = 'age';
ALTER ROLE api_runtime SET session_preload_libraries = 'age';

SELECT 'CREATE DATABASE heyblog OWNER postgres'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'heyblog')
\gexec

\connect heyblog

CREATE EXTENSION IF NOT EXISTS age;
COMMENT ON EXTENSION age IS 'Apache AGE provides the authoritative directed site friend-link graph.';

REVOKE ALL ON DATABASE heyblog FROM PUBLIC;
GRANT CONNECT, CREATE ON DATABASE heyblog TO migrator;
GRANT CONNECT ON DATABASE heyblog TO api_runtime;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA ag_catalog TO migrator;

CREATE SCHEMA IF NOT EXISTS migration AUTHORIZATION migrator;
ALTER SCHEMA migration OWNER TO migrator;
COMMENT ON SCHEMA migration IS 'Goose migration history owned by the migration role.';

\echo 'Initialized database heyblog and roles migrator and api_runtime.'

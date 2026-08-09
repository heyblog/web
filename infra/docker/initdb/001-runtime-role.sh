#!/bin/sh

set -eu

psql \
  --username "$POSTGRES_USER" \
  --dbname "$POSTGRES_DB" \
  --set=ON_ERROR_STOP=1 \
  --set=database_name="$POSTGRES_DB" \
  --set=migrator_password="$POSTGRES_MIGRATOR_PASSWORD" \
  --set=runtime_password="$POSTGRES_RUNTIME_PASSWORD" <<'SQL'
CREATE EXTENSION IF NOT EXISTS age;
COMMENT ON EXTENSION age IS 'Apache AGE provides the authoritative directed site friend-link graph.';

CREATE ROLE migrator
  LOGIN
  PASSWORD :'migrator_password'
  NOSUPERUSER
  NOCREATEDB
  NOCREATEROLE
  NOREPLICATION
  NOBYPASSRLS;
COMMENT ON ROLE migrator IS 'Owns migration metadata and application database objects without cluster administration privileges.';

CREATE ROLE api_runtime
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

REVOKE ALL ON DATABASE :"database_name" FROM PUBLIC;
GRANT CONNECT, CREATE ON DATABASE :"database_name" TO migrator;
GRANT CONNECT ON DATABASE :"database_name" TO api_runtime;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA ag_catalog TO migrator;

CREATE SCHEMA migration AUTHORIZATION migrator;
COMMENT ON SCHEMA migration IS 'Goose migration history owned by the migration role.';
SQL

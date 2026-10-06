//go:build integration

package integration_test

import (
	"context"
	"net/url"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func bootstrapDatabaseRoles(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()
	if _, err := connection.Exec(ctx, `
		CREATE EXTENSION IF NOT EXISTS age;
		COMMENT ON EXTENSION age IS
			'Apache AGE provides the authoritative directed site friend-link graph.';
		CREATE ROLE migrator
			LOGIN PASSWORD 'migrator-secret'
			NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
		CREATE ROLE api_runtime
			LOGIN PASSWORD 'runtime-secret'
			NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
		ALTER ROLE migrator SET session_preload_libraries = 'age';
		ALTER ROLE api_runtime SET session_preload_libraries = 'age';
		REVOKE ALL ON DATABASE heyblog FROM PUBLIC;
		GRANT CONNECT, CREATE ON DATABASE heyblog TO migrator;
		GRANT CONNECT ON DATABASE heyblog TO api_runtime;
		GRANT USAGE ON SCHEMA ag_catalog TO migrator;
		CREATE SCHEMA migration AUTHORIZATION migrator;
		COMMENT ON SCHEMA migration IS 'Goose migration history owned by the migration role.';
	`); err != nil {
		t.Fatalf("bootstrap database roles: %v", err)
	}
}

func verifyRoleBoundaries(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()
	for _, role := range []string{"migrator", "api_runtime"} {
		var canLogin, isSuperuser, canCreateDatabase, canCreateRole, canReplicate, canBypassRLS bool
		if err := connection.QueryRow(ctx, `
			SELECT rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls
			  FROM pg_roles
			 WHERE rolname = $1
		`, role).Scan(
			&canLogin,
			&isSuperuser,
			&canCreateDatabase,
			&canCreateRole,
			&canReplicate,
			&canBypassRLS,
		); err != nil {
			t.Fatalf("query %s role: %v", role, err)
		}
		if !canLogin || isSuperuser || canCreateDatabase || canCreateRole || canReplicate || canBypassRLS {
			t.Fatalf(
				"role %s attributes = login:%t super:%t createdb:%t createrole:%t replication:%t bypassrls:%t",
				role,
				canLogin,
				isSuperuser,
				canCreateDatabase,
				canCreateRole,
				canReplicate,
				canBypassRLS,
			)
		}
	}

	var migratorCanCreate, runtimeCanCreate, runtimeCanConnect, runtimeCanUseAGE bool
	if err := connection.QueryRow(ctx, `
		SELECT has_database_privilege('migrator', 'heyblog', 'CREATE'),
		       has_database_privilege('api_runtime', 'heyblog', 'CREATE'),
		       has_database_privilege('api_runtime', 'heyblog', 'CONNECT'),
		       has_schema_privilege('api_runtime', 'ag_catalog', 'USAGE')
	`).Scan(&migratorCanCreate, &runtimeCanCreate, &runtimeCanConnect, &runtimeCanUseAGE); err != nil {
		t.Fatalf("query database role privileges: %v", err)
	}
	if !migratorCanCreate || runtimeCanCreate || !runtimeCanConnect || runtimeCanUseAGE {
		t.Fatalf(
			"database privileges = migrator create:%t, runtime create:%t connect:%t AGE usage:%t",
			migratorCanCreate,
			runtimeCanCreate,
			runtimeCanConnect,
			runtimeCanUseAGE,
		)
	}
}

func verifyDatabaseCatalog(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()

	var extensionVersion string
	if err := connection.QueryRow(ctx, "SELECT extversion FROM pg_extension WHERE extname = 'age'").Scan(&extensionVersion); err != nil {
		t.Fatalf("query AGE extension: %v", err)
	}

	var businessSchemaCount int
	if err := connection.QueryRow(ctx, `
		SELECT count(*) FROM pg_namespace
		WHERE nspname = ANY($1::text[])
	`, []string{"identity", "directory", "content"}).Scan(&businessSchemaCount); err != nil {
		t.Fatalf("query business schemas: %v", err)
	}
	if businessSchemaCount != 3 {
		t.Fatalf("business schema count = %d, want 3", businessSchemaCount)
	}

	var tableCount int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM pg_class AS relation
		  JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
		 WHERE namespace.nspname = ANY($1::text[])
		   AND relation.relkind IN ('r', 'p')
	`, []string{"identity", "directory", "content"}).Scan(&tableCount); err != nil {
		t.Fatalf("query business tables: %v", err)
	}
	if tableCount != 33 {
		t.Fatalf("business table count = %d, want 33", tableCount)
	}

	var graphExists bool
	if err := connection.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM ag_catalog.ag_graph WHERE name = 'directory_graph')
	`).Scan(&graphExists); err != nil {
		t.Fatalf("query AGE graph: %v", err)
	}
	if !graphExists {
		t.Fatal("directory_graph AGE graph does not exist")
	}

	verifyCatalogComments(ctx, t, connection)
	verifyIdentitySchema(ctx, t, connection)
	verifyContentSchema(ctx, t, connection)
}

func verifyContentSchema(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()

	var revisionStatusColumns int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM information_schema.columns
		 WHERE table_schema = 'content'
		   AND table_name = 'announcement_revisions'
		   AND column_name = 'status'
	`).Scan(&revisionStatusColumns); err != nil {
		t.Fatalf("query announcement revision status column: %v", err)
	}
	if revisionStatusColumns != 0 {
		t.Fatalf("announcement revision status column count = %d, want 0", revisionStatusColumns)
	}

	wantConstraints := []string{
		"announcement_revisions_action_external_url_check",
		"announcement_revisions_action_label_check",
		"announcement_revisions_action_path_check",
	}
	rows, err := connection.Query(ctx, `
		SELECT constraint_name
		  FROM information_schema.table_constraints
		 WHERE table_schema = 'content'
		   AND table_name = 'announcement_revisions'
		   AND constraint_name = ANY($1::text[])
		 ORDER BY constraint_name
	`, wantConstraints)
	if err != nil {
		t.Fatalf("query announcement revision action constraints: %v", err)
	}
	constraints, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect announcement revision action constraints: %v", err)
	}
	if !slices.Equal(constraints, wantConstraints) {
		t.Fatalf("announcement revision action constraints = %v, want %v", constraints, wantConstraints)
	}
}

func verifyIdentitySchema(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()

	rows, err := connection.Query(ctx, `
		SELECT column_name || ':' || data_type || ':' || is_nullable
		  FROM information_schema.columns
		 WHERE table_schema = 'identity'
		   AND table_name = 'users'
		 ORDER BY ordinal_position
	`)
	if err != nil {
		t.Fatalf("query identity user columns: %v", err)
	}
	columns, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect identity user columns: %v", err)
	}

	want := []string{
		"id:uuid:NO",
		"email:text:YES",
		"username:text:NO",
		"display_name:text:NO",
		"password_hash:text:YES",
		"role:text:NO",
		"access_status:text:NO",
		"email_verified_at:timestamp with time zone:YES",
		"auth_version:integer:NO",
		"profile:jsonb:NO",
		"settings:jsonb:NO",
		"last_login_at:timestamp with time zone:YES",
		"deletion_requested_at:timestamp with time zone:YES",
		"deletion_scheduled_for:timestamp with time zone:YES",
		"deleted_at:timestamp with time zone:YES",
		"created_at:timestamp with time zone:NO",
		"updated_at:timestamp with time zone:NO",
	}
	if !slices.Equal(columns, want) {
		t.Fatalf("identity user columns = %v, want %v", columns, want)
	}
}

func verifyCatalogComments(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()
	schemas := []string{"identity", "directory", "content"}

	var undocumentedRelations int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM pg_class AS relation
		  JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
		  LEFT JOIN pg_description AS description
		    ON description.objoid = relation.oid AND description.objsubid = 0
		 WHERE namespace.nspname = ANY($1::text[])
		   AND relation.relkind IN ('r', 'v', 'p')
		   AND description.description IS NULL
	`, schemas).Scan(&undocumentedRelations); err != nil {
		t.Fatalf("query undocumented business relations: %v", err)
	}
	if undocumentedRelations != 0 {
		t.Fatalf("undocumented business relation count = %d, want 0", undocumentedRelations)
	}

	var undocumentedColumns int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM pg_class AS relation
		  JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
		  JOIN pg_attribute AS attribute
		    ON attribute.attrelid = relation.oid
		   AND attribute.attnum > 0
		   AND NOT attribute.attisdropped
		  LEFT JOIN pg_description AS description
		    ON description.objoid = relation.oid AND description.objsubid = attribute.attnum
		 WHERE namespace.nspname = ANY($1::text[])
		   AND relation.relkind IN ('r', 'v', 'p')
		   AND description.description IS NULL
	`, schemas).Scan(&undocumentedColumns); err != nil {
		t.Fatalf("query undocumented business columns: %v", err)
	}
	if undocumentedColumns != 0 {
		t.Fatalf("undocumented business column count = %d, want 0", undocumentedColumns)
	}

	var undocumentedFunctions int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM pg_proc AS routine
		  JOIN pg_namespace AS namespace ON namespace.oid = routine.pronamespace
		 WHERE namespace.nspname = ANY($1::text[])
		   AND obj_description(routine.oid, 'pg_proc') IS NULL
	`, schemas).Scan(&undocumentedFunctions); err != nil {
		t.Fatalf("query undocumented business functions: %v", err)
	}
	if undocumentedFunctions != 0 {
		t.Fatalf("undocumented business function count = %d, want 0", undocumentedFunctions)
	}

	var undocumentedTriggers int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM pg_trigger AS trigger
		  JOIN pg_class AS relation ON relation.oid = trigger.tgrelid
		  JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
		 WHERE namespace.nspname = ANY($1::text[])
		   AND NOT trigger.tgisinternal
		   AND obj_description(trigger.oid, 'pg_trigger') IS NULL
	`, schemas).Scan(&undocumentedTriggers); err != nil {
		t.Fatalf("query undocumented business triggers: %v", err)
	}
	if undocumentedTriggers != 0 {
		t.Fatalf("undocumented business trigger count = %d, want 0", undocumentedTriggers)
	}
}

func verifyRuntimePermissions(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()
	if _, err := connection.Exec(ctx, "CREATE SCHEMA forbidden_runtime_schema"); err == nil {
		t.Fatal("runtime role unexpectedly received schema DDL permission")
	}
	if _, err := connection.Exec(ctx, `SELECT * FROM directory_graph."SiteRef"`); err == nil {
		t.Fatal("runtime role unexpectedly received direct graph table access")
	}
}

func databaseURLForRole(t *testing.T, rawURL, username, password string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	parsed.User = url.UserPassword(username, password)
	return parsed.String()
}

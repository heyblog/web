//go:build integration

package integration_test

import (
	"context"
	"heyblog-api/internal/infrastructure/database"
	"heyblog-api/internal/infrastructure/database/migrations"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func verifyMigrationRollback(
	ctx context.Context,
	t *testing.T,
	adminConnection *pgx.Conn,
	migrationURL string,
) {
	t.Helper()
	migrationConfig, err := pgx.ParseConfig(migrationURL)
	if err != nil {
		t.Fatalf("parse migration URL for rollback: %v", err)
	}
	migrationDB := stdlib.OpenDB(*migrationConfig)
	defer func() { _ = migrationDB.Close() }()
	migrationFS, err := migrations.Filesystem()
	if err != nil {
		t.Fatalf("open migrations for rollback: %v", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		migrationDB,
		migrationFS,
		goose.WithTableName("migration.goose_db_version"),
	)
	if err != nil {
		t.Fatalf("create Goose rollback provider: %v", err)
	}
	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("roll migrations down to zero: %v", err)
	}

	var businessSchemaCount int
	if err := adminConnection.QueryRow(ctx, `
		SELECT count(*) FROM pg_namespace
		WHERE nspname = ANY($1::text[])
	`, []string{"identity", "directory", "content"}).Scan(&businessSchemaCount); err != nil {
		t.Fatalf("query business schemas after rollback: %v", err)
	}
	if businessSchemaCount != 0 {
		t.Fatalf("business schema count after rollback = %d, want 0", businessSchemaCount)
	}

	var graphExists bool
	if err := adminConnection.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM ag_catalog.ag_graph WHERE name = 'directory_graph')
	`).Scan(&graphExists); err != nil {
		t.Fatalf("query AGE graph after rollback: %v", err)
	}
	if graphExists {
		t.Fatal("directory_graph AGE graph remained after rollback")
	}

	if err := database.Migrate(ctx, migrationURL); err != nil {
		t.Fatalf("reapply migrations after rollback: %v", err)
	}
	verifyDatabaseCatalog(ctx, t, adminConnection)
}

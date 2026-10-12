//go:build integration

package integration_test

import (
	"context"
	"heyblog-api/internal/infrastructure/database/migrations"
	"strings"
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
	if _, err := provider.DownTo(ctx, 0); err == nil || !strings.Contains(err.Error(), "restore the previous database backup") {
		t.Fatalf("destructive downgrade must refuse: %v", err)
	}
}

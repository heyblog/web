package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"heyblog-api/internal/infrastructure/database/migrations"
)

const migrationTable = "migration.goose_db_version"

// Migrate applies every pending database migration while holding a PostgreSQL
// session advisory lock.
func Migrate(ctx context.Context, databaseURL string) (resultErr error) {
	connectionConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse migration database URL: %w", err)
	}

	db := sql.OpenDB(stdlib.GetConnector(*connectionConfig))
	defer func() { _ = db.Close() }()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect migration database: %w", err)
	}

	migrationFS, err := migrations.Filesystem()
	if err != nil {
		return fmt.Errorf("open migrations: %w", err)
	}

	sessionLocker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("create migration lock: %w", err)
	}
	connection, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open migration lock connection: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, connection.Close()) }()
	if err := sessionLocker.SessionLock(ctx, connection); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		resultErr = errors.Join(resultErr, sessionLocker.SessionUnlock(cleanup, connection))
	}()
	// Goose Up performs an unlocked HasPending first, which can create history.
	// Hold the session lock around both the lineage check and every Goose call.
	if err := ensureBaseline(ctx, connection); err != nil {
		return err
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrationFS,
		goose.WithTableName(migrationTable),
	)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

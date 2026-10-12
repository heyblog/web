package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const baselineIdentity = "heyblog-final-schema-3-2026-10-clean-seeds"

func ensureBaseline(ctx context.Context, connection *sql.Conn) error {
	var marked bool
	if err := connection.QueryRowContext(ctx, "SELECT to_regclass('migration.baseline') IS NOT NULL").Scan(&marked); err != nil {
		return fmt.Errorf("inspect database baseline: %w", err)
	}
	if marked {
		var identity string
		if err := connection.QueryRowContext(ctx, "SELECT identity FROM migration.baseline WHERE singleton").Scan(&identity); err != nil {
			return fmt.Errorf("read database baseline: %w", err)
		}
		if identity != baselineIdentity {
			return errors.New("unsupported database baseline; restore a compatible backup into a fresh database")
		}
		return nil
	}
	var occupied bool
	if err := connection.QueryRowContext(ctx, `SELECT
		EXISTS (SELECT 1 FROM pg_namespace WHERE nspname IN ('identity','directory','content','directory_graph','runtime_cache'))
		OR to_regclass('migration.goose_db_version') IS NOT NULL`).Scan(&occupied); err != nil {
		return fmt.Errorf("inspect existing database history: %w", err)
	}
	if occupied {
		return errors.New("legacy database history requires rebuilding; export with the previous application and restore into a fresh database")
	}
	_, err := connection.ExecContext(ctx, `
		CREATE TABLE migration.baseline (
			singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
			identity text NOT NULL
		);
		COMMENT ON TABLE migration.baseline IS 'Identifies the fresh-install migration lineage independently of Goose versions.';
		COMMENT ON COLUMN migration.baseline.singleton IS 'Restricts the lineage marker to one row.';
		COMMENT ON COLUMN migration.baseline.identity IS 'Stable baseline identity retained while migrations are resumed.';
		INSERT INTO migration.baseline(identity) VALUES ('heyblog-final-schema-3-2026-10-clean-seeds');
	`)
	if err != nil {
		return fmt.Errorf("initialize database baseline: %w", err)
	}
	return nil
}

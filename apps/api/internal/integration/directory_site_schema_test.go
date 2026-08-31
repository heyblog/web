//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyDirectorySiteTimestampSchema(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	// Given
	var joinedAtColumns, createdAtColumns int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE column_name = 'joined_at'),
		       count(*) FILTER (WHERE column_name = 'created_at')
		  FROM information_schema.columns
		 WHERE table_schema = 'directory'
		   AND table_name = 'sites'
	`).Scan(&joinedAtColumns, &createdAtColumns); err != nil {
		t.Fatalf("query directory site timestamp columns: %v", err)
	}

	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin default joined_at check: %v", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()

	// When
	var joinedAt time.Time
	if err := transaction.QueryRow(ctx, `
		INSERT INTO directory.sites (short_id, name, normalized_host)
		VALUES ('9Zy8Xw7Vu', 'Timestamp Contract', 'timestamp-contract.example.com')
		RETURNING joined_at
	`).Scan(&joinedAt); err != nil {
		t.Fatalf("insert site with default joined_at: %v", err)
	}

	// Then
	if joinedAtColumns != 1 || createdAtColumns != 0 {
		t.Fatalf("directory site timestamp column counts = (%d joined_at, %d created_at), want (1, 0)", joinedAtColumns, createdAtColumns)
	}
	if joinedAt.IsZero() {
		t.Fatal("directory site default joined_at is zero")
	}
	t.Logf("directory.sites timestamp columns: joined_at=%d created_at=%d; default joined_at=%s", joinedAtColumns, createdAtColumns, joinedAt.Format(time.RFC3339Nano))
}

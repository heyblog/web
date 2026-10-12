//go:build integration

package integration_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Capture complete rows, including seed IDs, timestamps, administrator data and
// AGE identifiers. Export intentionally omits some of these fields and therefore
// cannot prove that a failed transaction preserved the exact original state.
func databaseBackupSnapshot(t *testing.T, fixture auditMigrationFixture) map[string][]string {
	t.Helper()
	rows, err := fixture.admin.Query(t.Context(), `SELECT DISTINCT dataset FROM directory.backup_columns()
		UNION ALL SELECT 'directory_graph.'||c.relname FROM pg_class c
		JOIN pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname='directory_graph' AND c.relkind IN ('r','p') ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	snapshot := make(map[string][]string, len(names))
	businessTables := 0
	for _, name := range names {
		if !strings.HasPrefix(name, "directory_graph.") {
			businessTables++
		}
		identifier := pgx.Identifier(strings.SplitN(name, ".", 2)).Sanitize()
		rows, err := fixture.admin.Query(t.Context(), "SELECT to_jsonb(r)::text FROM ONLY "+identifier+" r ORDER BY to_jsonb(r)::text")
		if err != nil {
			t.Fatalf("snapshot %s: %v", name, err)
		}
		data, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		snapshot[name] = data
	}
	if businessTables != 33 || len(names) <= businessTables {
		t.Fatal("rollback snapshot requires all 33 business tables and graph tables")
	}
	return snapshot
}

func assertDatabaseBackupSnapshot(t *testing.T, fixture auditMigrationFixture, before map[string][]string) {
	t.Helper()
	after := databaseBackupSnapshot(t, fixture)
	if len(before) != len(after) {
		t.Fatal("restore changed the table inventory")
	}
	for name, rows := range before {
		if !reflect.DeepEqual(rows, after[name]) {
			t.Errorf("restore changed original rows in %s", name)
		}
	}
}

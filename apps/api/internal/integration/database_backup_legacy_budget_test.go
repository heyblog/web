//go:build integration

package integration_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"heyblog-api/internal/features/databasebackup"

	"github.com/stretchr/testify/require"
)

type legacyBudgetTransfer struct {
	Metadata databasebackup.Header       `json:"metadata"`
	Tables   map[string][]map[string]any `json:"tables"`
	Graph    map[string][]map[string]any `json:"graph"`
	Manifest []databasebackup.Manifest   `json:"manifest"`
}

func (data legacyBudgetTransfer) file(t *testing.T) *os.File {
	t.Helper()
	data.Manifest = nil
	appendDataset := func(name string, rows []map[string]any) {
		hash := sha256.New()
		for _, row := range rows {
			raw, err := json.Marshal(row)
			require.NoError(t, err)
			_, err = hash.Write(append(raw, '\n'))
			require.NoError(t, err)
		}
		data.Manifest = append(data.Manifest, databasebackup.Manifest{Dataset: databasebackup.Dataset{Name: name, Count: int64(len(rows))}, SHA256: hex.EncodeToString(hash.Sum(nil))})
	}
	for name, rows := range data.Tables {
		appendDataset(name, rows)
	}
	for name, rows := range data.Graph {
		appendDataset("graph."+name, rows)
	}
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "legacy-budget.json")
	require.NoError(t, os.WriteFile(path, raw, 0600))
	file, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })
	return file
}

func TestDatabaseBackupRejectsIntermediateFormats(t *testing.T) {
	fixture := newAuditMigrationFixture(t)
	_, err := fixture.provider.Up(t.Context())
	require.NoError(t, err)
	var admin string
	require.NoError(t, fixture.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name,role)
 VALUES('format@example.test','format_admin','Format','SYS_ADMIN') RETURNING id::text`).Scan(&admin))
	service := databasebackup.New(fixture.pool)
	exported, err := service.Export(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { _ = exported.Close(); _ = os.Remove(exported.Name()) })
	var current legacyBudgetTransfer
	decoder := json.NewDecoder(exported)
	decoder.UseNumber()
	require.NoError(t, decoder.Decode(&current))
	require.Equal(t, 9, current.Metadata.SchemaVersion)
	current.Metadata.SchemaVersion = 8
	_, err = service.Inspect(t.Context(), current.file(t), admin)
	require.ErrorIs(t, err, databasebackup.ErrInvalid)
	current.Metadata.SchemaVersion = 9
	current.Tables["operations.platform_budgets"] = []map[string]any{}
	_, err = service.Inspect(t.Context(), current.file(t), admin)
	require.ErrorIs(t, err, databasebackup.ErrInvalid)
	delete(current.Tables, "operations.platform_budgets")
	// A correctly checksummed row with the old task shape must also be rejected.
	current.Tables["operations.runs"] = []map[string]any{{"id": "019f033c-2111-7000-9000-000000000001", "state": "completed"}}
	_, err = service.Inspect(t.Context(), current.file(t), admin)
	require.ErrorIs(t, err, databasebackup.ErrInvalid)
}

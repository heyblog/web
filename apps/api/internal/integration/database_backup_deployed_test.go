//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"heyblog-api/internal/features/databasebackup"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestDatabaseBackupRestoresDeployedSchemas(t *testing.T) {
	// Given rows exported by the deployed schema-six SQL contract.
	source := newAuditMigrationFixture(t)
	_, err := source.provider.UpTo(t.Context(), 16)
	require.NoError(t, err)
	var admin, siteID string
	require.NoError(t, source.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name,role)
		VALUES('deployed@example.test','deployed_admin','Deployed','SYS_ADMIN') RETURNING id::text`).Scan(&admin))
	require.NoError(t, source.pool.QueryRow(t.Context(), `INSERT INTO directory.sites
		(short_id,name,scheme,normalized_host,base_path,access_scope,visibility)
		VALUES('Deployed1','Deployed site','https','deployed.example.test','/','GLOBAL_ONLY','VISIBLE') RETURNING id::text`).Scan(&siteID))
	var names []string
	require.NoError(t, source.pool.QueryRow(t.Context(), `SELECT array_agg(DISTINCT dataset ORDER BY dataset) FROM directory.backup_columns()`).Scan(&names))
	for _, version := range []int{3, 4, 5, 6} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			data := legacyBudgetTransfer{Metadata: databasebackup.Header{Format: databasebackup.Format,
				Version: 1, SchemaVersion: version, GeneratedAt: time.Now().UTC(), ExcludedSystemAdminID: admin},
				Tables: map[string][]map[string]any{}, Graph: map[string][]map[string]any{}}
			for _, name := range names {
				if version < 6 && name == "directory.site_claim_attachments" {
					continue
				}
				rows, err := source.pool.Query(t.Context(), `SELECT directory.backup_table_rows($1,$2::uuid)`, name, admin)
				require.NoError(t, err)
				data.Tables[name] = deployedBackupRows(t, rows)
				for _, row := range data.Tables[name] {
					if version == 3 && name == "identity.users" {
						delete(row, "profile_confirmation_status")
						delete(row, "username_changes_used")
					}
					if version < 6 && name == "directory.site_claims" {
						delete(row, "reviewed_at")
					}
				}
			}
			for _, kind := range []string{"vertices", "edges"} {
				rows, err := source.pool.Query(t.Context(), `SELECT directory.backup_graph_rows($1)`, kind)
				require.NoError(t, err)
				data.Graph[kind] = deployedBackupRows(t, rows)
			}
			// When inspecting and restoring into the final schema.
			target := newAuditMigrationFixture(t)
			_, err := target.provider.Up(t.Context())
			require.NoError(t, err)
			var retained string
			require.NoError(t, target.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name,role)
				VALUES('retained@example.test','retained_admin','Retained','SYS_ADMIN') RETURNING id::text`).Scan(&retained))
			service := databasebackup.New(target.pool)
			file := data.file(t)
			preview, err := service.Inspect(t.Context(), file, retained)
			require.NoError(t, err)
			require.True(t, preview.CanRestore, "%+v", preview.Issues)
			_, err = service.Restore(t.Context(), file, retained, preview.SHA256)
			require.NoError(t, err)
			// Then content identifiers survive and monitoring starts without stale conclusions.
			var preserved bool
			require.NoError(t, target.pool.QueryRow(t.Context(), `SELECT name='Deployed site' AND access_scope IS NULL
				AND status IS NULL AND monitoring_generation=1 FROM directory.sites WHERE id=$1`, siteID).Scan(&preserved))
			require.True(t, preserved)
			var enabled int
			require.NoError(t, target.pool.QueryRow(t.Context(), `SELECT count(*) FROM operations.request_configs WHERE enabled`).Scan(&enabled))
			require.Zero(t, enabled)
		})
	}
}

func deployedBackupRows(t *testing.T, rows pgx.Rows) []map[string]any {
	t.Helper()
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var raw []byte
		require.NoError(t, rows.Scan(&raw))
		var row map[string]any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		require.NoError(t, decoder.Decode(&row))
		result = append(result, row)
	}
	require.NoError(t, rows.Err())
	return result
}

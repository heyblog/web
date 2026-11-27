//go:build integration

package integration_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"heyblog-api/internal/features/databasebackup"

	"github.com/stretchr/testify/require"
)

type backupArchive struct {
	Metadata databasebackup.Header       `json:"metadata"`
	Tables   map[string][]map[string]any `json:"tables"`
	Graph    map[string][]map[string]any `json:"graph"`
	Manifest []databasebackup.Manifest   `json:"manifest"`
}

func TestDatabaseBackupRestoresLegacySiteClassification(t *testing.T) {
	ctx := t.Context()
	source := newAuditMigrationFixture(t)
	_, err := source.provider.UpTo(ctx, 16)
	require.NoError(t, err)
	_, err = source.pool.Exec(ctx, `
		INSERT INTO identity.users(email,username,display_name,role)
		VALUES ('source@example.test','source_admin','Source','SYS_ADMIN');
		INSERT INTO identity.users(email,username,display_name)
		VALUES ('oauth@example.test','oauth_backup','OAuth'),('local@example.test','local_backup','Local');
		INSERT INTO identity.oauth_identities(user_id,provider,provider_user_id)
		SELECT id,'GITHUB','backup-oauth-id' FROM identity.users WHERE username='oauth_backup';
		INSERT INTO directory.sites(short_id,name,normalized_host,revision,updated_at)
		SELECT short_id, short_id, lower(short_id) || '.example.test', 9007199254740993, '2024-02-01T03:04:05Z'
		  FROM unnest(ARRAY['Legacy001','Legacy002','Legacy003','Legacy004']) AS short_id;
		INSERT INTO directory.site_audits(
			lookup_secret_hash,action,status,site_id,base_revision,base_snapshot,proposed_snapshot,
			final_snapshot,request_reason,reviewed_by,reviewed_at
		)
		SELECT decode(repeat(example.hash,32),'hex'),example.action,example.status,site.id,
		       CASE WHEN example.action <> 'CREATE' THEN 1 END,
		       CASE WHEN example.action <> 'CREATE' THEN '{}'::jsonb END, '{}'::jsonb,
		       CASE WHEN example.status = 'APPROVED' THEN '{}'::jsonb END,'Historical review',
		       CASE WHEN example.status = 'APPROVED' THEN reviewer.id END,
		       CASE WHEN example.status = 'APPROVED' THEN '2024-01-31'::timestamptz END
		  FROM (VALUES ('Legacy001','CREATE','APPROVED','aa'),('Legacy002','UPDATE','APPROVED','bb'),
		               ('Legacy003','DELETE','APPROVED','cc'),('Legacy004','UPDATE','PENDING','dd'))
		       AS example(short_id,action,status,hash)
		  JOIN directory.sites AS site ON site.short_id = example.short_id
		 CROSS JOIN identity.users AS reviewer WHERE reviewer.role = 'SYS_ADMIN';
	`)
	require.NoError(t, err)
	_, err = source.provider.Up(ctx)
	require.NoError(t, err)
	want := []string{
		"Legacy001|BLOG|9007199254740993|2024-02-01T03:04:05Z",
		"Legacy002|BLOG|9007199254740993|2024-02-01T03:04:05Z",
		"Legacy003|UNREVIEWED|9007199254740993|2024-02-01T03:04:05Z",
		"Legacy004|UNREVIEWED|9007199254740993|2024-02-01T03:04:05Z",
	}
	require.Equal(t, want, legacyBackupSiteState(t, source))

	exported, err := databasebackup.New(source.pool).Export(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = exported.Close(); _ = os.Remove(exported.Name()) })
	for version := 3; version <= 7; version++ {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			file := legacyBackupFile(t, exported, version)
			target := newAuditMigrationFixture(t)
			_, err := target.provider.Up(t.Context())
			require.NoError(t, err)
			var retained string
			require.NoError(t, target.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name,role)
				VALUES('target@example.test','target_admin','Target','SYS_ADMIN') RETURNING id::text`).Scan(&retained))
			restore := databasebackup.New(target.pool)
			preview, err := restore.Inspect(t.Context(), file, retained)
			require.NoError(t, err)
			require.True(t, preview.CanRestore)
			require.Equal(t, version, preview.SchemaVersion)
			_, err = restore.Restore(t.Context(), file, retained, preview.SHA256)
			require.NoError(t, err)
			require.Equal(t, want, legacyBackupSiteState(t, target))
			var historyCount int
			require.NoError(t, target.pool.QueryRow(t.Context(), "SELECT count(*) FROM directory.site_annotation_history").Scan(&historyCount))
			require.Zero(t, historyCount)
			if version == 3 {
				assertLegacyBackupAccountProfiles(t, target)
			}
		})
	}
}

func legacyBackupFile(t *testing.T, exported *os.File, version int) *os.File {
	t.Helper()
	_, err := exported.Seek(0, 0)
	require.NoError(t, err)
	decoder := json.NewDecoder(exported)
	decoder.UseNumber()
	var archive backupArchive
	require.NoError(t, decoder.Decode(&archive))
	archive.Metadata.SchemaVersion = version
	if version < 7 {
		delete(archive.Tables, "directory.site_annotation_history")
		for _, row := range archive.Tables["directory.sites"] {
			delete(row, "blog_status")
		}
	}
	for _, row := range archive.Tables["directory.site_claims"] {
		delete(row, "email_change_id")
		delete(row, "email_change_ownership_id")
	}
	if version < 6 {
		delete(archive.Tables, "directory.site_claim_attachments")
		for _, row := range archive.Tables["directory.site_claims"] {
			delete(row, "reviewed_at")
		}
	}
	if version == 3 {
		for _, row := range archive.Tables["identity.users"] {
			delete(row, "profile_confirmation_status")
			delete(row, "username_changes_used")
		}
	}
	manifest := archive.Manifest[:0]
	for _, entry := range archive.Manifest {
		if entry.Name == "graph.vertices" || entry.Name == "graph.edges" {
			manifest = append(manifest, entry)
			continue
		}
		rows, included := archive.Tables[entry.Name]
		if !included {
			continue
		}
		hash := sha256.New()
		for _, row := range rows {
			raw, err := json.Marshal(row)
			require.NoError(t, err)
			_, err = hash.Write(append(raw, '\n'))
			require.NoError(t, err)
		}
		entry.SHA256 = hex.EncodeToString(hash.Sum(nil))
		manifest = append(manifest, entry)
	}
	archive.Manifest = manifest
	file, err := os.CreateTemp(t.TempDir(), "legacy-*.json")
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })
	require.NoError(t, json.NewEncoder(file).Encode(archive))
	return file
}

func legacyBackupSiteState(t *testing.T, fixture auditMigrationFixture) []string {
	t.Helper()
	var states []string
	require.NoError(t, fixture.pool.QueryRow(t.Context(), `
		SELECT array_agg(concat_ws('|',short_id,blog_status,revision,
		       to_char(updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')) ORDER BY short_id)
		  FROM directory.sites
	`).Scan(&states))
	return states
}

func assertLegacyBackupAccountProfiles(t *testing.T, target auditMigrationFixture) {
	t.Helper()
	for _, test := range []struct{ username, state string }{{"oauth_backup", "PENDING"}, {"local_backup", "NOT_REQUIRED"}} {
		var state string
		var used int
		require.NoError(t, target.pool.QueryRow(t.Context(), `SELECT profile_confirmation_status,username_changes_used
			FROM identity.users WHERE username=$1`, test.username).Scan(&state, &used))
		require.Equal(t, test.state, state)
		require.Zero(t, used)
	}
}

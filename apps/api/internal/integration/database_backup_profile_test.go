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
)

func TestDatabaseBackupConvertsVersionThreeAccountProfiles(t *testing.T) {
	source := newAuditMigrationFixture(t)
	if _, err := source.provider.Up(t.Context()); err != nil {
		t.Fatal(err)
	}
	var admin string
	if err := source.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name,role) VALUES('source@example.test','source_admin','Source','SYS_ADMIN') RETURNING id::text`).Scan(&admin); err != nil {
		t.Fatal(err)
	}
	if _, err := source.pool.Exec(t.Context(), `INSERT INTO identity.users(email,username,display_name) VALUES ('oauth@example.test','oauth_backup','OAuth'),('local@example.test','local_backup','Local'); INSERT INTO identity.oauth_identities(user_id,provider,provider_user_id) SELECT id,'GITHUB','backup-oauth-id' FROM identity.users WHERE username='oauth_backup'`); err != nil {
		t.Fatal(err)
	}
	exported, err := databasebackup.New(source.pool).Export(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = exported.Close(); _ = os.Remove(exported.Name()) })
	var data struct {
		Metadata databasebackup.Header       `json:"metadata"`
		Tables   map[string][]map[string]any `json:"tables"`
		Graph    map[string][]map[string]any `json:"graph"`
		Manifest []databasebackup.Manifest   `json:"manifest"`
	}
	decoder := json.NewDecoder(exported)
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		t.Fatal(err)
	}
	data.Metadata.SchemaVersion = 3
	delete(data.Tables, "directory.site_claim_attachments")
	filtered := data.Manifest[:0]
	for _, entry := range data.Manifest {
		if entry.Name != "directory.site_claim_attachments" {
			filtered = append(filtered, entry)
		}
	}
	data.Manifest = filtered
	for _, row := range data.Tables["directory.site_claims"] {
		delete(row, "reviewed_at")
	}

	hash := sha256.New()
	for _, row := range data.Tables["identity.users"] {
		delete(row, "profile_confirmation_status")
		delete(row, "username_changes_used")
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := hash.Write(append(raw, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	for i := range data.Manifest {
		if data.Manifest[i].Name == "identity.users" {
			data.Manifest[i].SHA256 = hex.EncodeToString(hash.Sum(nil))
		}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "schema-three.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	target := newAuditMigrationFixture(t)
	if _, err := target.provider.Up(t.Context()); err != nil {
		t.Fatal(err)
	}
	var retained string
	if err := target.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name,role) VALUES('target@example.test','target_admin','Target','SYS_ADMIN') RETURNING id::text`).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	restore := databasebackup.New(target.pool)
	preview, err := restore.Inspect(t.Context(), file, retained)
	if err != nil || !preview.CanRestore || preview.SchemaVersion != 3 {
		t.Fatalf("preview %+v / %v", preview, err)
	}
	if _, err := restore.Restore(t.Context(), file, retained, preview.SHA256); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ username, state string }{{"oauth_backup", "PENDING"}, {"local_backup", "NOT_REQUIRED"}} {
		var state string
		var used int
		if err := target.pool.QueryRow(t.Context(), `SELECT profile_confirmation_status,username_changes_used FROM identity.users WHERE username=$1`, test.username).Scan(&state, &used); err != nil {
			t.Fatal(err)
		}
		if state != test.state || used != 0 {
			t.Fatalf("%s state=%s used=%d", test.username, state, used)
		}
	}
}

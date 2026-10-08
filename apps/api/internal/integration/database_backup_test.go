//go:build integration

package integration_test

import (
	"errors"
	"heyblog-api/internal/features/apikey"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"heyblog-api/internal/features/databasebackup"
)

func TestDatabaseBackupRestoresOriginalIdentifiers(t *testing.T) {
	source := newAuditMigrationFixture(t)
	if _, err := source.provider.Up(t.Context()); err != nil {
		t.Fatal(err)
	}
	var fingerprint string
	if err := source.admin.QueryRow(t.Context(), "SELECT directory.backup_seed_fingerprint()").Scan(&fingerprint); err != nil {
		t.Fatal(err)
	}
	t.Logf("initialization seed fingerprint: %s", fingerprint)
	var schemaFingerprint string
	if err := source.admin.QueryRow(t.Context(), `SELECT md5(string_agg(n.nspname||'.'||c.relname||':'||a.attname||':'||a.atttypid::regtype::text||':'||a.attgenerated::text,',' ORDER BY n.nspname,c.relname,a.attnum)) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped WHERE n.nspname IN ('identity','directory','content') AND c.relkind IN ('r','p')`).Scan(&schemaFingerprint); err != nil {
		t.Fatal(err)
	}
	t.Logf("backup schema fingerprint: %s", schemaFingerprint)
	var actor string
	if err := source.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name,role) VALUES('source@example.test','source_admin','Source','SYS_ADMIN') RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	siteID := insertSite(t.Context(), t, source.pool, "Backup123", "Backup site", "backup.example.test")
	credential := seedDatabaseBackup(t, source, actor, siteID)
	service := databasebackup.New(source.pool)
	file, err := service.Export(t.Context())
	if err != nil {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) {
			t.Fatalf("export failed: %#v", pgerr)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close(); _ = os.Remove(file.Name()) })
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
	if err != nil {
		t.Fatal(err)
	}
	if !preview.CanRestore {
		t.Fatalf("target rejected: %+v", preview.Issues)
	}
	verifyBackupRestoreLocks(t, target, retained)
	verifyBackupRestoreRollback(t, target, restore, file, retained, preview.SHA256)
	result, err := restore.Restore(t.Context(), file, retained, preview.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if result.AdminMapping.SourceID != actor || result.AdminMapping.TargetID != retained {
		t.Fatal("administrator mapping mismatch")
	}
	var actual string
	if err := target.pool.QueryRow(t.Context(), `SELECT id::text FROM directory.sites WHERE short_id='Backup123'`).Scan(&actual); err != nil {
		t.Fatal(err)
	}
	if actual != integrationUUIDText(t, siteID) {
		t.Fatal("business identifier changed")
	}
	reexport, err := restore.Export(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reexport.Close(); _ = os.Remove(reexport.Name()) })
	compareDatabaseBackups(t, file, reexport, actor, retained)
	keys := apikey.NewService(apikey.NewRepository(target.pool), time.Now)
	principal, err := keys.Authenticate(t.Context(), credential.Token, apikey.AccessPolicy{Audiences: []apikey.Audience{apikey.AudienceInternal}, Scope: apikey.ScopeExampleCall})
	if err != nil || principal.KeyID != credential.Key.ID {
		t.Fatalf("restored API credential failed: %v", err)
	}
}

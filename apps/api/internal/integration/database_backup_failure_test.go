//go:build integration

package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"heyblog-api/internal/features/databasebackup"
)

func verifyBackupRestoreRollback(t *testing.T, target auditMigrationFixture, service *databasebackup.Service, file *os.File, actor, checksum string) {
	t.Helper()
	ctx := t.Context()
	before := databaseBackupSnapshot(t, target)
	if _, err := target.admin.Exec(ctx, `CREATE FUNCTION public.reject_backup_setting() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic restore failure'; END $$; CREATE TRIGGER reject_backup_setting BEFORE INSERT ON content.announcement_revisions FOR EACH ROW EXECUTE FUNCTION public.reject_backup_setting()`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Restore(ctx, file, actor, checksum); err == nil {
		t.Fatal("mid-restore failure unexpectedly committed")
	}
	assertDatabaseBackupSnapshot(t, target, before)
	if _, err := target.admin.Exec(ctx, `DROP TRIGGER reject_backup_setting ON content.announcement_revisions; DROP FUNCTION public.reject_backup_setting()`); err != nil {
		t.Fatal(err)
	}
}

func verifyBackupRestoreLocks(t *testing.T, target auditMigrationFixture, actor string) {
	t.Helper()
	ctx := t.Context()
	if _, err := target.pool.Exec(ctx, `SELECT set_config('heyblog.database_restore',txid_current()::text,true); SELECT directory.backup_restore_row('identity.users','{}',uuidv7(),uuidv7())`); err == nil {
		t.Fatal("runtime forged restore authority")
	}
	tx, err := target.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT directory.backup_restore_begin($1::uuid)", actor); err != nil {
		t.Fatal(err)
	}
	var runtimeAuthorized bool
	if err := tx.QueryRow(ctx, "SELECT directory.backup_restore_is_active()").Scan(&runtimeAuthorized); err != nil {
		t.Fatal(err)
	}
	if runtimeAuthorized {
		t.Fatal("runtime caller gained migrator authority")
	}
	readCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	var retained string
	if err := target.pool.QueryRow(readCtx, `SELECT u.id::text FROM identity.users u
		LEFT JOIN identity.oauth_identities o ON o.user_id=u.id
		LEFT JOIN identity.user_management_permissions p ON p.user_id=u.id
		WHERE u.id=$1`, actor).Scan(&retained); err != nil || retained != actor {
		t.Fatalf("session identity read during restore = %q, %v", retained, err)
	}
	for _, table := range []string{"identity.users", "identity.oauth_identities", "identity.user_management_permissions"} {
		writer, err := target.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, lockErr := writer.Exec(ctx, "LOCK TABLE "+table+" IN ROW EXCLUSIVE MODE NOWAIT")
		if err := writer.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		if lockErr == nil {
			t.Fatalf("concurrent write lock accepted for %s", table)
		}
	}
	if _, err := target.pool.Exec(ctx, "SELECT directory.backup_restore_begin($1::uuid)", actor); err == nil {
		t.Fatal("concurrent restore accepted")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}

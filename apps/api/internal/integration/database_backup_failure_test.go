//go:build integration

package integration_test

import (
	"os"
	"testing"

	"heyblog-api/internal/features/databasebackup"
)

func verifyBackupRestoreRollback(t *testing.T, target auditMigrationFixture, service *databasebackup.Service, file *os.File, actor, checksum string) {
	t.Helper()
	ctx := t.Context()
	var before string
	if err := target.admin.QueryRow(ctx, "SELECT directory.backup_seed_fingerprint()").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := target.admin.Exec(ctx, `CREATE FUNCTION public.reject_backup_setting() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic restore failure'; END $$; CREATE TRIGGER reject_backup_setting BEFORE INSERT ON content.announcement_revisions FOR EACH ROW EXECUTE FUNCTION public.reject_backup_setting()`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Restore(ctx, file, actor, checksum); err == nil {
		t.Fatal("mid-restore failure unexpectedly committed")
	}
	var after string
	if err := target.admin.QueryRow(ctx, "SELECT directory.backup_seed_fingerprint()").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("failed restore changed initialization seeds")
	}
	var users, sites, clients int
	if err := target.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM identity.users),(SELECT count(*) FROM directory.sites),(SELECT count(*) FROM identity.api_clients)`).Scan(&users, &sites, &clients); err != nil {
		t.Fatal(err)
	}
	if users != 1 || sites != 0 || clients != 0 {
		t.Fatal("failed restore retained partial data")
	}
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
	if _, err := target.pool.Exec(ctx, "SELECT directory.backup_restore_begin($1::uuid)", actor); err == nil {
		t.Fatal("concurrent restore accepted")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}

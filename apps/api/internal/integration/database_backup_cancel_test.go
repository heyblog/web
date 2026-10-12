//go:build integration

package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"heyblog-api/internal/features/databasebackup"
)

func verifyBackupRestoreCancellation(t *testing.T, target auditMigrationFixture, service *databasebackup.Service, file *os.File, actor, checksum string) {
	t.Helper()
	for _, scenario := range []struct {
		name    string
		stage   string
		dataset string
	}{
		{name: "taxonomy", stage: "restoring", dataset: "directory.tag_cascades"},
		{name: "graph", stage: "restoring", dataset: "graph.edges"},
		{name: "final_validation", stage: "checking"},
		{name: "before_commit", stage: "committing"},
	} {
		t.Run("cancel_"+scenario.name, func(t *testing.T) {
			before := databaseBackupSnapshot(t, target)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			interrupted := false
			var siteLocks int
			_, err := service.RestoreWithProgress(ctx, file, actor, checksum, func(progress databasebackup.RestoreProgress) error {
				if progress.Stage == "restoring" && (progress.Dataset == "directory.sites" || progress.Dataset == "directory.site_feeds") {
					var locks int
					if err := target.admin.QueryRow(t.Context(), "SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND database=(SELECT oid FROM pg_database WHERE datname=current_database())").Scan(&locks); err != nil {
						return err
					}
					if progress.Dataset == "directory.sites" && siteLocks == 0 {
						siteLocks = locks
					} else if locks != siteLocks {
						t.Errorf("restore lock count grew with site rows: before=%d after=%d", siteLocks, locks)
					}
				}
				if progress.Stage == scenario.stage && progress.Dataset == scenario.dataset {
					interrupted = true
					cancel()
					return ctx.Err()
				}
				return nil
			})
			if !interrupted || !errors.Is(err, context.Canceled) || errors.Is(err, databasebackup.ErrOutcomeUnknown) {
				t.Fatalf("restore cancellation did not confirm rollback: interrupted=%t error=%v", interrupted, err)
			}
			assertDatabaseBackupSnapshot(t, target, before)
		})
	}
}

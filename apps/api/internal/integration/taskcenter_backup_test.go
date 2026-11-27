//go:build integration

package integration_test

import (
	"bytes"
	"os"
	"testing"
	"time"

	"heyblog-api/internal/domain/proof"
	"heyblog-api/internal/features/databasebackup"
	"heyblog-api/internal/features/monitoring"
	"heyblog-api/internal/features/taskcenter"

	"heyblog.local/taskprotocol"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTaskCenterBackupResetsExecutionAndSigning(t *testing.T) {
	// Given an active signing key, an unfinished leased task and real observations.
	source := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := source.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var actor string
	if err := source.pool.QueryRow(ctx, `INSERT INTO identity.users(email,username,display_name,role) VALUES('backup-task@example.test','backup_task','Backup','SYS_ADMIN') RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	site := integrationUUIDText(t, insertSite(ctx, t, source.pool, "BackupTsk", "Backup task", "backup-task.example.test"))
	repository := taskcenter.NewRepository(source.pool, time.Now)
	signing := taskcenter.NewSigning(repository, make([]byte, 32))
	key, err := signing.Generate(ctx, actor, proof.Cloudflare)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"publish", "activate"} {
		if err := signing.Transition(ctx, actor, key, action); err != nil {
			t.Fatal(err)
		}
	}
	schedules, err := repository.Schedules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var schedule taskcenter.Schedule
	for _, s := range schedules {
		if s.Kind == "monitor" {
			schedule = s
		}
	}
	schedule.Enabled = true
	schedule, err = repository.Save(ctx, taskAdministrator, schedule.ID, schedule.ScheduleInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Manual(ctx, taskAdministrator, schedule.ID); err != nil {
		t.Fatal(err)
	}
	worker := taskprotocol.WorkerInput{InstanceID: uuid.NewString(), Protocol: taskprotocol.ProtocolVersion}
	if _, err := repository.Register(ctx, taskprotocol.Instance{InstanceID: worker.InstanceID, Protocol: worker.Protocol, Version: "integration"}); err != nil {
		t.Fatal(err)
	}
	lease, err := repository.Claim(ctx, worker)
	if err != nil || lease == nil {
		t.Fatalf("claim: %v", err)
	}
	var endedRun, endedTarget, statistic string
	require.NoError(t, source.pool.QueryRow(ctx, `INSERT INTO operations.runs(kind,state,snapshot,scheduled_at,finished_at,source)
		VALUES('monitor','succeeded','{}',now(),now(),'manual') RETURNING id::text`).Scan(&endedRun))
	require.NoError(t, source.pool.QueryRow(ctx, `INSERT INTO operations.targets(run_id,site_id,kind,address,generation,state,finished_at,usable,result)
		VALUES($1,$2,'monitor','https://backup-task.example.test/',1,'succeeded',now(),true,'{"business_valid":true,"paths":[]}')
		RETURNING id::text`, endedRun, site).Scan(&endedTarget))
	statistic = uuid.NewString()
	_, err = source.pool.Exec(ctx, `INSERT INTO operations.request_statistics(attempt_id,platform,kind,authorized,sent,received,succeeded,duration_ms)
		VALUES($1,'CF','monitor',true,true,true,true,123)`, statistic)
	require.NoError(t, err)
	_, err = source.pool.Exec(ctx, `UPDATE operations.platform_backoff SET retry_at=now()+interval '1 hour',reason='platform_rate_limited',stage=2,
		probe_id=$1,probe_until=now()+interval '1 minute' WHERE platform='CF'`, uuid.NewString())
	require.NoError(t, err)
	for index := range 10 {
		if err := monitoring.New(source.pool).Record(ctx, monitoring.Round{SiteID: site, Generation: 1, ID: time.Duration(index).String(), StartedAt: time.Now().Add(time.Duration(index) * time.Second), CN: monitoring.OK, GL: monitoring.OK}); err != nil {
			t.Fatal(err)
		}
	}
	var ciphertext []byte
	if err := source.pool.QueryRow(ctx, `SELECT encrypted_private_key FROM operations.signing_keys WHERE id=$1`, key).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	file, err := databasebackup.New(source.pool).Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close(); _ = os.Remove(file.Name()) })
	target := newAuditMigrationFixture(t)
	if _, err := target.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var retained string
	if err := target.pool.QueryRow(ctx, `INSERT INTO identity.users(email,username,display_name,role) VALUES('restored-task@example.test','restored_task','Restored','SYS_ADMIN') RETURNING id::text`).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	restore := databasebackup.New(target.pool)
	preview, err := restore.Inspect(ctx, file, retained)
	if err != nil || !preview.CanRestore {
		t.Fatalf("inspect: %+v %v", preview, err)
	}
	// When restoring schema 7 into an initialized database.
	require.Equal(t, 9, preview.SchemaVersion)
	if _, err := restore.Restore(ctx, file, retained, preview.SHA256); err != nil {
		t.Fatal(err)
	}
	// Then key bytes survive encrypted, but execution capabilities are invalidated.
	var restoredCiphertext []byte
	var state string
	if err := target.pool.QueryRow(ctx, `SELECT encrypted_private_key,state FROM operations.signing_keys WHERE id=$1`, key).Scan(&restoredCiphertext, &state); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ciphertext, restoredCiphertext) || state != "revoked" {
		t.Fatal("restored signing key was enabled or modified")
	}
	var reset bool
	if err := target.pool.QueryRow(ctx, `SELECT state='cancelled' AND lease_token IS NULL AND lease_until IS NULL FROM operations.targets WHERE id=$1`, lease.ID).Scan(&reset); err != nil || !reset {
		t.Fatalf("lease reset: %v %v", reset, err)
	}
	if err := target.pool.QueryRow(ctx, `SELECT state='cancelled' FROM operations.runs WHERE id=$1`, lease.RunID).Scan(&reset); err != nil || !reset {
		t.Fatalf("run reset: %v %v", reset, err)
	}
	if err := target.pool.QueryRow(ctx, `SELECT NOT enabled AND next_at IS NULL FROM operations.schedules WHERE id=$1`, schedule.ID).Scan(&reset); err != nil || !reset {
		t.Fatalf("schedule reset: %v %v", reset, err)
	}
	if err := target.pool.QueryRow(ctx, `SELECT status IS NULL AND access_scope IS NULL AND last_checked_at IS NULL AND monitoring_generation>1 FROM directory.sites WHERE id=$1`, site).Scan(&reset); err != nil || !reset {
		t.Fatalf("monitoring reset: %v %v", reset, err)
	}
	var rounds int
	if err := target.pool.QueryRow(ctx, `SELECT count(*) FROM directory.site_monitoring_rounds`).Scan(&rounds); err != nil || rounds != 0 {
		t.Fatalf("rounds=%d %v", rounds, err)
	}
	if _, err := taskcenter.NewRepository(target.pool, time.Now).Heartbeat(ctx, lease.ID, taskprotocol.Fence{WorkerInput: worker, Token: lease.Token}); err == nil {
		t.Fatal("restored lease accepted")
	}
	require.NoError(t, target.pool.QueryRow(ctx, `SELECT r.state='succeeded' AND t.state='succeeded' AND t.usable
		AND t.result->>'business_valid'='true' FROM operations.runs r JOIN operations.targets t ON t.run_id=r.id
		WHERE r.id=$1 AND t.id=$2`, endedRun, endedTarget).Scan(&reset))
	require.True(t, reset)
	require.NoError(t, target.pool.QueryRow(ctx, `SELECT authorized AND sent AND received AND succeeded AND NOT unknown AND duration_ms=123
		FROM operations.request_statistics WHERE attempt_id=$1`, statistic).Scan(&reset))
	require.True(t, reset)
	require.NoError(t, target.pool.QueryRow(ctx, `SELECT bool_and(NOT enabled AND parameters->>'signed'='true') FROM operations.request_configs`).Scan(&reset))
	require.True(t, reset)
	require.NoError(t, target.pool.QueryRow(ctx, `SELECT draining AND heartbeat_at='1970-01-01'::timestamptz FROM operations.workers WHERE id=$1`, worker.InstanceID).Scan(&reset))
	require.True(t, reset)
	require.NoError(t, target.pool.QueryRow(ctx, `SELECT bool_and(retry_at IS NULL AND reason='' AND stage=0 AND probe_id IS NULL AND probe_until IS NULL)
		FROM operations.platform_backoff`).Scan(&reset))
	require.True(t, reset)
}

package databasebackup

import (
	"context"
	"errors"
	"os"
	"testing"
)

type restoreFunc func(context.Context, *os.File, string, string, func(RestoreProgress) error) (Restoration, error)

func (restore restoreFunc) RestoreWithProgress(ctx context.Context, file *os.File, actor, checksum string, report func(RestoreProgress) error) (Restoration, error) {
	return restore(ctx, file, actor, checksum, report)
}

func startTestJob(t *testing.T, jobs *Jobs, ctx context.Context) (RestoreJob, string) {
	t.Helper()
	if err := jobs.reserve(); err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(t.TempDir(), "restore-job-*")
	if err != nil {
		t.Fatal(err)
	}
	job, err := jobs.start(ctx, file, "actor", "checksum")
	if err != nil {
		t.Fatal(err)
	}
	return job, file.Name()
}

func TestRestoreJobsDetachRequestAndHoldSlotUntilCleanup(t *testing.T) {
	t.Parallel()
	started, release := make(chan struct{}), make(chan struct{})
	jobs := NewJobs(restoreFunc(func(ctx context.Context, _ *os.File, _, _ string, _ func(RestoreProgress) error) (Restoration, error) {
		close(started)
		<-release
		if err := ctx.Err(); err != nil {
			return Restoration{}, err
		}
		return Restoration{Status: "restored"}, nil
	}), nil)
	t.Cleanup(jobs.Close)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	job, path := startTestJob(t, jobs, ctx)
	<-started
	cancel()
	if err := jobs.reserve(); !errors.Is(err, ErrBusy) {
		t.Fatalf("reservation: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	close(release)
	jobs.worker.Wait()
	got, err := jobs.Get(job.ID)
	if err != nil || got.Status != "succeeded" {
		t.Fatalf("job=%+v err=%v", got, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file retained: %v", err)
	}
	if err := jobs.reserve(); err != nil {
		t.Fatal(err)
	}
	jobs.release()
}

func TestRestoreJobCancellationWaitsForRollback(t *testing.T) {
	t.Parallel()
	rollingBack, finishRollback := make(chan struct{}), make(chan struct{})
	jobs := NewJobs(restoreFunc(func(ctx context.Context, _ *os.File, _, _ string, report func(RestoreProgress) error) (Restoration, error) {
		<-ctx.Done()
		if err := report(RestoreProgress{Stage: "rolling_back"}); err != nil {
			return Restoration{}, err
		}
		close(rollingBack)
		<-finishRollback
		return Restoration{}, ctx.Err()
	}), nil)
	t.Cleanup(jobs.Close)
	job, _ := startTestJob(t, jobs, t.Context())
	if _, err := jobs.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	<-rollingBack
	got, err := jobs.Get(job.ID)
	if err != nil || got.Status != "cancelling" || got.Progress.Stage != "rolling_back" {
		t.Fatalf("job=%+v err=%v", got, err)
	}
	if err := jobs.reserve(); !errors.Is(err, ErrBusy) {
		t.Fatalf("reservation released before rollback: %v", err)
	}
	close(finishRollback)
	jobs.worker.Wait()
	got, err = jobs.Get(job.ID)
	if err != nil || got.Status != "cancelled" {
		t.Fatalf("job=%+v err=%v", got, err)
	}
}

func TestRestoreJobCommitGateWinsLateCancellation(t *testing.T) {
	t.Parallel()
	committing, finish := make(chan struct{}), make(chan struct{})
	jobs := NewJobs(restoreFunc(func(ctx context.Context, _ *os.File, _, _ string, report func(RestoreProgress) error) (Restoration, error) {
		if err := report(RestoreProgress{Stage: "committing"}); err != nil {
			return Restoration{}, err
		}
		close(committing)
		<-finish
		if err := ctx.Err(); err != nil {
			return Restoration{}, err
		}
		return Restoration{Status: "restored"}, nil
	}), nil)
	t.Cleanup(jobs.Close)
	job, _ := startTestJob(t, jobs, t.Context())
	<-committing
	got, err := jobs.Cancel(job.ID)
	if err != nil || got.Status != "running" {
		t.Fatalf("job=%+v err=%v", got, err)
	}
	close(finish)
	jobs.worker.Wait()
	got, err = jobs.Get(job.ID)
	if err != nil || got.Status != "succeeded" {
		t.Fatalf("job=%+v err=%v", got, err)
	}
}

func TestRestoreJobsCloseCancelsAndJoinsWorker(t *testing.T) {
	t.Parallel()
	finished := make(chan struct{})
	jobs := NewJobs(restoreFunc(func(ctx context.Context, _ *os.File, _, _ string, _ func(RestoreProgress) error) (Restoration, error) {
		<-ctx.Done()
		close(finished)
		return Restoration{}, errors.Join(ErrOutcomeUnknown, ctx.Err())
	}), nil)
	job, path := startTestJob(t, jobs, t.Context())
	jobs.Close()
	select {
	case <-finished:
	default:
		t.Fatal("close returned before worker")
	}
	got, err := jobs.Get(job.ID)
	if err != nil || got.Status != "unknown" {
		t.Fatalf("job=%+v err=%v", got, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file retained: %v", err)
	}
	if err := jobs.reserve(); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed reservation: %v", err)
	}
}

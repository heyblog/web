package databasebackup

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrBusy           = errors.New("database restore is already active")
	ErrJobNotFound    = errors.New("database restore task not found")
	ErrClosed         = errors.New("database restore manager is closed")
	ErrOutcomeUnknown = errors.New("database restore outcome is unknown")
)

type RestoreProgress struct {
	Stage     string `json:"stage" enum:"validating,restoring,checking,committing,rolling_back"`
	Dataset   string `json:"dataset"`
	Completed int64  `json:"completed" minimum:"0"`
	Total     int64  `json:"total" minimum:"0"`
}

type RestoreJob struct {
	UploadID  string          `json:"upload_id,omitempty"`
	ID        string          `json:"id" format:"uuid"`
	Status    string          `json:"status" enum:"running,cancelling,succeeded,failed,cancelled,unknown"`
	Progress  RestoreProgress `json:"progress"`
	Result    *Restoration    `json:"result,omitempty"`
	ErrorCode string          `json:"error_code,omitempty"`
}

type Restorer interface {
	RestoreWithProgress(context.Context, *os.File, string, string, func(RestoreProgress) error) (Restoration, error)
}

// Jobs owns the sole upload reservation and the latest restore for this process.
// Close joins the worker before bootstrap closes its database pool.
type Jobs struct {
	mu      sync.Mutex
	worker  sync.WaitGroup
	service Restorer
	logger  *slog.Logger
	current *RestoreJob
	cancel  context.CancelFunc
	busy    bool
	closed  bool
}

func NewJobs(service Restorer, logger *slog.Logger) *Jobs {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Jobs{service: service, logger: logger}
}

func (jobs *Jobs) reserve() error {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	if jobs.closed {
		return ErrClosed
	}
	if jobs.busy {
		return ErrBusy
	}
	jobs.busy = true
	return nil
}

func (jobs *Jobs) release() {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	jobs.busy = false
}

// start transfers file ownership after upload and confirmation have succeeded.
func (jobs *Jobs) start(ctx context.Context, file *os.File, actor, checksum string) (RestoreJob, error) {
	return jobs.startOwned(ctx, file, actor, checksum, "", uuid.NewString(), nil)
}

func (jobs *Jobs) startOwned(ctx context.Context, file *os.File, actor, checksum, uploadID, jobID string, completed func(RestoreJob)) (RestoreJob, error) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	if jobs.closed {
		return RestoreJob{}, ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return RestoreJob{}, err
	}
	work, cancel := context.WithTimeout(context.WithoutCancel(ctx), Timeout)
	jobs.cancel = cancel
	jobs.current = &RestoreJob{ID: jobID, Status: "running", Progress: RestoreProgress{Stage: "validating"}}
	jobs.current.UploadID = uploadID
	initial := *jobs.current
	jobs.worker.Add(1)
	go jobs.run(work, cancel, file, actor, checksum, completed)
	return initial, nil
}

func (jobs *Jobs) run(ctx context.Context, cancel context.CancelFunc, file *os.File, actor, checksum string, completed func(RestoreJob)) {
	defer jobs.worker.Done()
	defer cancel()
	result, err := jobs.service.RestoreWithProgress(ctx, file, actor, checksum, func(progress RestoreProgress) error {
		jobs.mu.Lock()
		defer jobs.mu.Unlock()
		if progress.Stage == "rolling_back" {
			jobs.current.Progress = progress
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		jobs.current.Progress = progress
		return nil
	})
	if cleanupErr := errorsWithCleanup(nil, file); cleanupErr != nil {
		jobs.logger.Warn("database backup temporary file cleanup failed", "event", "database_backup_cleanup_failed")
	}
	jobs.mu.Lock()
	defer func() {
		result := jobs.snapshot()
		jobs.mu.Unlock()
		if completed != nil {
			completed(*result)
		}
	}()
	jobs.busy = false
	jobs.cancel = nil
	switch {
	case err == nil:
		jobs.current.Status, jobs.current.Result = "succeeded", &result
		jobs.logger.Info("database restore completed", "event", "database_restore_completed", "job_id", jobs.current.ID)
	case errors.Is(err, ErrOutcomeUnknown):
		jobs.current.Status, jobs.current.ErrorCode = "unknown", "restore_outcome_unknown"
	case errors.Is(err, context.Canceled):
		jobs.current.Status, jobs.current.ErrorCode = "cancelled", "database_backup_cancelled"
	default:
		jobs.current.Status, jobs.current.ErrorCode = "failed", restoreErrorCode(err)
	}
	if err != nil && jobs.current.Status != "cancelled" {
		var databaseError *pgconn.PgError
		args := []any{"event", "database_restore_failed", "job_id", jobs.current.ID, "code", jobs.current.ErrorCode}
		if errors.As(err, &databaseError) {
			args = append(args, "sqlstate", databaseError.Code)
		}
		jobs.logger.Error("database restore failed", args...)
	}
}

func restoreErrorCode(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "database_backup_timeout"
	case errors.Is(err, ErrInvalid):
		return "database_backup_invalid"
	case errors.Is(err, ErrTarget):
		return "database_backup_target_not_ready"
	default:
		return "database_backup_restore_failed"
	}
}

func (jobs *Jobs) Current() *RestoreJob {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	return jobs.snapshot()
}

func (jobs *Jobs) snapshot() *RestoreJob {
	if jobs.current == nil {
		return nil
	}
	result := *jobs.current
	if result.Result != nil {
		restoration := *result.Result
		restoration.Datasets = append([]Dataset(nil), restoration.Datasets...)
		result.Result = &restoration
	}
	return &result
}

func (jobs *Jobs) Get(id string) (RestoreJob, error) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	if jobs.current == nil || jobs.current.ID != id {
		return RestoreJob{}, ErrJobNotFound
	}
	return *jobs.snapshot(), nil
}

func (jobs *Jobs) Cancel(id string) (RestoreJob, error) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	if jobs.current == nil || jobs.current.ID != id {
		return RestoreJob{}, ErrJobNotFound
	}
	jobs.cancelLocked()
	return *jobs.snapshot(), nil
}

func (jobs *Jobs) cancelLocked() {
	if jobs.cancel != nil && jobs.current.Progress.Stage != "committing" {
		jobs.current.Status = "cancelling"
		jobs.cancel()
	}
}

func (jobs *Jobs) Close() {
	jobs.mu.Lock()
	jobs.closed = true
	jobs.cancelLocked()
	jobs.mu.Unlock()
	jobs.worker.Wait()
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

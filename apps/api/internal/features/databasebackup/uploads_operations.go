package databasebackup

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/google/uuid"
)

func (uploads *Uploads) acquire(ctx context.Context, owner, id string, restore bool) (*uploadEntry, *os.File, error) {
	entry, err := uploads.entry(owner, id)
	if err != nil {
		return nil, nil, err
	}
	if entry.leased || entry.worker {
		return nil, nil, ErrBusy
	}
	if entry.record.State != "ready" && entry.record.State != "inspected" {
		return nil, nil, ErrUploadState
	}
	if restore && (entry.record.State != "inspected" || entry.record.Inspection == nil || !entry.record.Inspection.CanRestore) {
		return nil, nil, ErrUploadState
	}
	file, err := os.Open(uploads.localPath(id)) // #nosec G304 -- ID is bound to the server-created upload record.
	if errors.Is(err, os.ErrNotExist) {
		if uploads.settings.Transport == "direct" {
			resetDirect(&entry.record)
			return nil, nil, errors.Join(ErrUploadState, uploads.metadata.Save(ctx, entry.record))
		}
		entry.record.State, entry.record.PreparedBytes = "preparing", 0
		if err := uploads.metadata.Save(ctx, entry.record); err != nil {
			return nil, nil, err
		}
		uploads.startPreparation(entry)
		return nil, nil, ErrUploadState
	}
	if err != nil {
		return nil, nil, err
	}
	entry.leased = true
	return entry, file, nil
}

func (uploads *Uploads) Inspect(ctx context.Context, owner, id string, service Operations, jobs *Jobs) (Inspection, error) {
	if err := jobs.reserve(); err != nil {
		return Inspection{}, err
	}
	defer jobs.release()
	uploads.mu.Lock()
	entry, file, err := uploads.acquire(ctx, owner, id, false)
	if err != nil {
		uploads.mu.Unlock()
		return Inspection{}, err
	}
	entry.record.State = "inspecting"
	entry.record.touch(uploads.now())
	err = uploads.metadata.Save(ctx, entry.record)
	if err != nil {
		entry.leased = false
		entry.record.State = "ready"
		uploads.mu.Unlock()
		return Inspection{}, errors.Join(err, file.Close())
	}
	uploads.mu.Unlock()
	preview, inspectErr := service.Inspect(ctx, file, owner)
	closeErr := file.Close()
	uploads.mu.Lock()
	defer uploads.mu.Unlock()
	entry.leased = false
	entry.record.State = "ready"
	if inspectErr == nil {
		entry.record.State, entry.record.Inspection = "inspected", &preview
	} else if errors.Is(inspectErr, ErrInvalid) || errors.Is(inspectErr, ErrTooLarge) {
		entry.record.State, entry.record.ErrorCode, entry.record.CleanupStatus = "failed", "database_backup_invalid", "pending"
		uploads.wakeCleanup()
	}
	entry.record.touch(uploads.now())
	return preview, errors.Join(inspectErr, closeErr, uploads.saveDetached(entry.record))
}

func (uploads *Uploads) Restore(ctx context.Context, owner, id, checksum string, serviceJobs *Jobs) (RestoreJob, error) {
	if err := serviceJobs.reserve(); err != nil {
		return RestoreJob{}, err
	}
	accepted := false
	defer func() {
		if !accepted {
			serviceJobs.release()
		}
	}()
	uploads.mu.Lock()
	defer uploads.mu.Unlock()
	entry, file, err := uploads.acquire(ctx, owner, id, true)
	if err != nil {
		return RestoreJob{}, err
	}
	if checksum != entry.record.SHA256 || entry.record.Inspection.SHA256 != checksum {
		entry.leased = false
		return RestoreJob{}, errors.Join(ErrInvalid, file.Close())
	}
	previous := entry.record
	entry.record.State = "restoring"
	jobID := uuid.NewString()
	initial := RestoreJob{ID: jobID, UploadID: id, Status: "running", Progress: RestoreProgress{Stage: "validating"}}
	entry.record.JobID, entry.record.Job = jobID, &initial
	entry.record.touch(uploads.now())
	if err := uploads.metadata.Save(ctx, entry.record); err != nil {
		entry.leased, entry.record = false, previous
		return RestoreJob{}, errors.Join(err, file.Close())
	}
	job, err := serviceJobs.startOwned(ctx, file, owner, checksum, id, jobID, func(result RestoreJob) { uploads.restored(id, result) })
	if err != nil {
		entry.leased, entry.record = false, previous
		return RestoreJob{}, errors.Join(err, file.Close(), uploads.saveDetached(entry.record))
	}
	entry.record.JobID, entry.record.Job = job.ID, &job
	// Once accepted, never turn a metadata outage into an ambiguous retryable
	// restore response. The pre-start restoring intent already survives restart.
	if err := uploads.saveDetached(entry.record); err != nil {
		uploads.report("metadata", id)
	}
	accepted = true
	return job, nil
}

func (uploads *Uploads) restored(id string, job RestoreJob) {
	uploads.mu.Lock()
	defer uploads.mu.Unlock()
	entry := uploads.entries[id]
	entry.leased = false
	entry.record.State, entry.record.Job, entry.record.ErrorCode = job.Status, &job, job.ErrorCode
	entry.record.CleanupStatus = "pending"
	if err := uploads.saveDetached(entry.record); err != nil {
		uploads.report("metadata", id)
	}
	uploads.wakeCleanup()
}

func (uploads *Uploads) StoredJob(owner, id string) (*RestoreJob, error) {
	if !uploads.settings.Enabled {
		return nil, ErrJobNotFound
	}
	uploads.mu.Lock()
	defer uploads.mu.Unlock()
	var current *uploadRecord
	for _, entry := range uploads.entries {
		if entry.record.Owner == owner && entry.record.Job != nil && (id == "" || entry.record.Job.ID == id) && (current == nil || entry.record.CreatedAt.After(current.CreatedAt)) {
			record := entry.record
			current = &record
		}
	}
	if current == nil {
		return nil, ErrJobNotFound
	}
	job := *current.Job
	return &job, nil
}

func operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 30*time.Minute)
}

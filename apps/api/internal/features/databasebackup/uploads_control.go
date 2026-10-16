package databasebackup

import "context"

// extendRemoteUncertainty is called while holding uploads.mu. A failed remote
// request may still be running in COS, so its grace starts when failure is seen.
func (uploads *Uploads) extendRemoteUncertainty(record *uploadRecord) {
	if deadline := uploads.now().Add(Timeout); deadline.After(record.RemoteUncertainUntil) {
		record.RemoteUncertainUntil = deadline
	}
}

// control starts request-bound COS work while holding uploads.mu. The caller
// releases that mutex around network I/O and calls finish with it held again.
// This keeps DELETE and shutdown able to cancel slow control-plane requests.
func (uploads *Uploads) control(ctx context.Context, entry *uploadEntry) (context.Context, func(), error) {
	if entry.worker || entry.leased || len(entry.writers) > 0 {
		return nil, nil, ErrBusy
	}
	work, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(uploads.ctx, cancel)
	entry.worker, entry.cancel = true, cancel
	uploads.workers.Add(1)
	return work, func() {
		stop()
		cancel()
		entry.worker, entry.cancel = false, nil
		uploads.workers.Done()
		if entry.record.CleanupStatus == "pending" {
			uploads.wakeCleanup()
		}
	}, nil
}

func (uploads *Uploads) listParts(ctx context.Context, entry *uploadEntry) ([]remotePart, error) {
	work, finish, err := uploads.control(ctx, entry)
	if err != nil {
		return nil, err
	}
	defer finish()
	record := entry.record
	uploads.mu.Unlock()
	var parts []remotePart
	if uploads.settings.Transport == "direct" {
		parts, err = uploads.directParts(work, record)
	} else {
		parts, err = uploads.storage.Parts(work, record.Key, record.RemoteUploadID)
	}
	uploads.mu.Lock()
	if err != nil {
		return nil, err
	}
	if entry.record.State != "uploading" && (uploads.settings.Transport != "direct" || entry.record.State != "preparation_failed") {
		return nil, ErrUploadState
	}
	return parts, nil
}

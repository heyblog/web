package databasebackup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (uploads *Uploads) Delete(ctx context.Context, owner, id string) (UploadSession, error) {
	uploads.mu.Lock()
	defer uploads.mu.Unlock()
	entry, err := uploads.entry(owner, id)
	if err != nil {
		return UploadSession{}, err
	}
	if entry.record.State == "restoring" || entry.leased {
		return UploadSession{}, ErrBusy
	}
	if entry.record.CleanupStatus == "completed" {
		return entry.record.session(), nil
	}
	if !uploadTerminal(entry.record.State) {
		entry.record.State = "cancelled"
	}
	entry.record.CleanupStatus = "pending"
	if err := uploads.metadata.Save(ctx, entry.record); err != nil {
		return UploadSession{}, err
	}
	if entry.cancel != nil {
		entry.cancel()
	}
	for _, cancel := range entry.writers {
		cancel()
	}
	uploads.wakeCleanup()
	return entry.record.session(), nil
}

func (uploads *Uploads) sweepLoop() {
	defer uploads.workers.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		uploads.sweep()
		select {
		case <-uploads.ctx.Done():
			return
		case <-ticker.C:
		case <-uploads.wake:
		}
	}
}

func (uploads *Uploads) sweep() {
	uploads.mu.Lock()
	ids := make([]string, 0, len(uploads.entries))
	for id := range uploads.entries {
		ids = append(ids, id)
	}
	uploads.mu.Unlock()
	for _, id := range ids {
		if uploads.ctx.Err() != nil {
			return
		}
		uploads.sweepEntry(id)
	}
}

func (uploads *Uploads) sweepEntry(id string) {
	uploads.mu.Lock()
	entry := uploads.entries[id]
	if entry == nil || entry.leased || entry.worker || len(entry.writers) > 0 {
		uploads.mu.Unlock()
		return
	}
	now := uploads.now()
	if !uploadTerminal(entry.record.State) && !now.Before(entry.record.ExpiresAt) {
		entry.record.State, entry.record.CleanupStatus = "expired", "pending"
	}
	if entry.record.CleanupStatus == "completed" && !entry.record.CompletedAt.IsZero() && now.Sub(entry.record.CompletedAt) >= uploadTombstoneTTL {
		if err := uploads.metadata.Delete(uploads.ctx, id); err != nil {
			uploads.report("metadata_cleanup", id)
		} else {
			delete(uploads.entries, id)
		}
		uploads.mu.Unlock()
		return
	}
	if entry.record.CleanupStatus != "pending" {
		// Retry a previous metadata write failure without dropping live intent.
		if err := uploads.metadata.Save(uploads.ctx, entry.record); err != nil {
			uploads.report("metadata", id)
		}
		uploads.mu.Unlock()
		return
	}
	if now.Before(entry.record.RemoteUncertainUntil) {
		uploads.mu.Unlock()
		return
	}
	if err := uploads.metadata.Save(uploads.ctx, entry.record); err != nil {
		uploads.report("metadata", id)
		uploads.mu.Unlock()
		return
	}
	entry.worker = true
	key := entry.record.Key
	uploads.mu.Unlock()
	ctx, cancel := context.WithTimeout(uploads.ctx, time.Minute)
	var err error
	if uploads.settings.Transport == "cos" {
		err = uploads.storage.Cleanup(ctx, key)
	}
	cancel()
	err = errors.Join(err, uploads.removeLocal(id))
	uploads.mu.Lock()
	defer uploads.mu.Unlock()
	entry.worker = false
	if err != nil {
		uploads.report("cleanup", id)
		return
	}
	entry.record.CleanupStatus, entry.record.CompletedAt = "completed", uploads.now()
	if err := uploads.saveDetached(entry.record); err != nil {
		uploads.report("metadata", id)
	}
}

func (uploads *Uploads) report(stage, id string) {
	uploads.logger.Warn("database backup upload operation failed", "event", "database_backup_upload_operation_failed", "upload_id", id, "stage", stage)
}

func (uploads *Uploads) wakeCleanup() {
	select {
	case uploads.wake <- struct{}{}:
	default:
	}
}

func (uploads *Uploads) pruneLocalOrphans() error {
	files, err := os.ReadDir(uploads.directory)
	if err != nil {
		return err
	}
	for _, file := range files {
		name := strings.TrimSuffix(file.Name(), ".partial")
		if !strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".parts") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimSuffix(name, ".json"), ".parts")
		parsed, err := uuid.Parse(id)
		if err != nil || parsed.String() != id {
			continue
		}
		entry := uploads.entries[id]
		if entry == nil || entry.record.CleanupStatus == "completed" {
			if err := os.RemoveAll(filepath.Join(uploads.directory, file.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func (uploads *Uploads) Close() {
	if !uploads.settings.Enabled {
		return
	}
	uploads.mu.Lock()
	uploads.closed = true
	uploads.cancel()
	uploads.mu.Unlock()
	uploads.workers.Wait()
	// Direct data has no remote copy. Preserve active sessions across restart.
	uploads.mu.Lock()
	defer uploads.mu.Unlock()
	for id, entry := range uploads.entries {
		if entry.leased || uploads.settings.Transport == "direct" && !uploadTerminal(entry.record.State) {
			continue
		}
		if err := uploads.removeLocal(id); err != nil {
			uploads.report("shutdown_cleanup", id)
		}
	}
}

func (uploads *Uploads) removeLocal(id string) error {
	var result error
	for _, path := range []string{uploads.localPath(id), uploads.localPath(id) + ".partial"} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, err)
		}
	}
	return errors.Join(result, os.RemoveAll(uploads.partsDirectory(id)))
}

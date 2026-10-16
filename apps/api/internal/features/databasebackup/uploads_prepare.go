package databasebackup

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

func (uploads *Uploads) Complete(ctx context.Context, owner, id string) (UploadSession, error) {
	uploads.mu.Lock()
	defer uploads.mu.Unlock()
	entry, err := uploads.entry(owner, id)
	if err != nil {
		return UploadSession{}, err
	}
	switch entry.record.State {
	case "preparing", "ready", "inspecting", "inspected", "restoring", "succeeded":
		return entry.record.session(), nil
	case "uploading", "preparation_failed":
		if entry.worker || len(entry.writers) > 0 {
			return UploadSession{}, ErrBusy
		}
	default:
		return UploadSession{}, ErrUploadState
	}
	if uploads.settings.Transport == "direct" {
		parts, err := uploads.listParts(ctx, entry)
		if err != nil {
			return UploadSession{}, err
		}
		if err := reconcileParts(&entry.record, parts, true); err != nil {
			return UploadSession{}, err
		}
	}
	previous := entry.record
	entry.record.State, entry.record.ErrorCode, entry.record.PreparedBytes = "preparing", "", 0
	entry.record.touch(uploads.now())
	if err := uploads.metadata.Save(ctx, entry.record); err != nil {
		entry.record = previous
		return UploadSession{}, err
	}
	uploads.startPreparation(entry)
	return entry.record.session(), nil
}

// startPreparation is called while holding uploads.mu.
func (uploads *Uploads) startPreparation(entry *uploadEntry) {
	ctx, cancel := context.WithTimeout(uploads.ctx, Timeout)
	entry.cancel, entry.worker = cancel, true
	record := entry.record
	uploads.workers.Add(1)
	go func() {
		defer uploads.workers.Done()
		defer cancel()
		checksum, err := uploads.prepare(ctx, record)
		uploads.mu.Lock()
		defer uploads.mu.Unlock()
		entry.worker, entry.cancel = false, nil
		if entry.record.State == "preparing" {
			switch {
			case err == nil:
				entry.record.State, entry.record.SHA256, entry.record.PreparedBytes = "ready", checksum, record.Size
				entry.record.UploadedBytes = record.Size
				entry.record.UploadedParts = make([]int, len(record.PartSHA256))
				for index := range entry.record.UploadedParts {
					entry.record.UploadedParts[index] = index + 1
				}
			case errors.Is(err, ErrInvalid):
				entry.record.State, entry.record.ErrorCode, entry.record.CleanupStatus = "failed", "database_backup_invalid", "pending"
			default:
				entry.record.State, entry.record.ErrorCode = "preparation_failed", "database_backup_preparation_failed"
			}
		}
		if err := uploads.saveDetached(entry.record); err != nil {
			uploads.report("metadata", entry.record.ID)
		}
		if entry.record.CleanupStatus == "pending" {
			uploads.wakeCleanup()
		}
		if err != nil {
			uploads.report("prepare", entry.record.ID)
		}
	}()
}

func (uploads *Uploads) prepare(ctx context.Context, record uploadRecord) (string, error) {
	if uploads.settings.Transport == "direct" {
		return uploads.prepareDirect(ctx, record)
	}
	object := record.Object
	uncertainUntil := record.RemoteUncertainUntil
	if object.ETag == "" {
		var err error
		object, err = uploads.storage.Head(ctx, record.Key)
		if errors.Is(err, errRemoteMissing) {
			parts, partsErr := uploads.storage.Parts(ctx, record.Key, record.RemoteUploadID)
			if partsErr != nil {
				return "", partsErr
			}
			if err := reconcileParts(&record, parts, true); err != nil {
				return "", err
			}
			uploads.mu.Lock()
			entry := uploads.entries[record.ID]
			uploads.extendRemoteUncertainty(&entry.record)
			persistErr := uploads.metadata.Save(ctx, entry.record)
			uploads.mu.Unlock()
			if persistErr != nil {
				return "", persistErr
			}
			object, err = uploads.storage.Complete(ctx, record.Key, record.RemoteUploadID, parts)
			if err != nil {
				uploads.mu.Lock()
				uploads.extendRemoteUncertainty(&entry.record)
				uncertainUntil = entry.record.RemoteUncertainUntil
				persistErr := uploads.saveDetached(entry.record)
				uploads.mu.Unlock()
				if persistErr != nil {
					return "", errors.Join(err, persistErr)
				}
				// Complete may have succeeded after the connection was lost.
				object, err = uploads.storage.Head(ctx, record.Key)
			}
		}
		if err != nil {
			return "", err
		}
		if object.Size != record.Size || object.ETag == "" {
			return "", ErrInvalid
		}
		uploads.mu.Lock()
		entry := uploads.entries[record.ID]
		entry.record.Object = object
		entry.record.RemoteUncertainUntil = uncertainUntil
		err = uploads.metadata.Save(ctx, entry.record)
		uploads.mu.Unlock()
		if err != nil {
			return "", err
		}
	}
	body, err := uploads.storage.Open(ctx, record.Key, object)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := body.Close(); err != nil {
			uploads.report("download_close", record.ID)
		}
	}()
	return uploads.prepareFile(ctx, record, body)
}

func (uploads *Uploads) localPath(id string) string {
	return filepath.Join(uploads.directory, id+".json")
}

func (uploads *Uploads) saveDetached(record uploadRecord) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(uploads.ctx), 10*time.Second)
	defer cancel()
	if err := uploads.metadata.Save(ctx, record); err != nil {
		return fmt.Errorf("save backup upload status: %w", err)
	}
	return nil
}
